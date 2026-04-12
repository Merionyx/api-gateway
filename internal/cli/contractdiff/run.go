package contractdiff

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"reflect"
	"sort"
	"strings"

	apiclient "github.com/merionyx/api-gateway/internal/cli/apiserver"
	"github.com/merionyx/api-gateway/internal/cli/convertfmt"
	"github.com/merionyx/api-gateway/internal/cli/style"

	"github.com/merionyx/go-diff"
)

// ErrChanges is returned when local and export differ (content or set of files).
var ErrChanges = errors.New("diff: changes detected")

// Options configures Compare.
type Options struct {
	// Target is a local file or directory (same role as export --out).
	Target     string
	ServerURL  string
	HTTPClient *http.Client
	Request    apiclient.ExportRequest
	Out        io.Writer
	Color      bool
}

// Summary aggregates compare outcome.
type Summary struct {
	Unchanged  int
	Changed    int
	OnlyLocal  int
	OnlyRemote int
}

// HasWork returns true if any pairwise diff or orphan side exists.
func (s Summary) HasWork() bool {
	return s.Changed > 0 || s.OnlyLocal > 0 || s.OnlyRemote > 0
}

type remoteEntry struct {
	File apiclient.ExportFile
	Doc  map[string]any
}

// Compare loads local files, fetches export, compares parsed documents with reflect.DeepEqual,
// and prints unified diffs of canonical YAML when content differs.
func Compare(ctx context.Context, o Options) (Summary, error) {
	local, err := LoadLocal(o.Target)
	if err != nil {
		return Summary{}, err
	}
	if o.HTTPClient == nil {
		o.HTTPClient = http.DefaultClient
	}
	remoteFiles, err := apiclient.ExportContracts(ctx, o.HTTPClient, o.ServerURL, o.Request)
	if err != nil {
		return Summary{}, err
	}

	remote := make(map[string]remoteEntry)
	for _, f := range remoteFiles {
		raw, err := apiclient.DecodePayload(f)
		if err != nil {
			return Summary{}, fmt.Errorf("decode %s: %w", f.ContractName, err)
		}
		ext, err := convertfmt.NormalizeExtFromSourcePath(f.SourcePath)
		if err != nil {
			return Summary{}, fmt.Errorf("contract %s: %w", f.ContractName, err)
		}
		doc, err := ParseDocument(raw, ext)
		if err != nil {
			return Summary{}, fmt.Errorf("contract %s: %w", f.ContractName, err)
		}
		if metaName, err := ContractNameFromDoc(doc); err == nil && metaName != f.ContractName {
			_, _ = fmt.Fprintf(o.Out, "%s metadata contract.name %q differs from export name %q (using export name as key)\n",
				style.S(o.Color, style.Dim, "note:"), metaName, f.ContractName)
		}
		remote[f.ContractName] = remoteEntry{File: f, Doc: doc}
	}

	var sum Summary
	names := unionKeys(local, remote)

	_, _ = fmt.Fprintln(o.Out)
	_, _ = fmt.Fprintln(o.Out, style.S(o.Color, style.Bold, "Diff contracts"))
	_, _ = fmt.Fprintln(o.Out)
	printKV(o.Out, o.Color, "Repository", o.Request.Repository)
	printKV(o.Out, o.Color, "Ref", o.Request.Ref)
	if strings.TrimSpace(o.Request.Path) != "" {
		printKV(o.Out, o.Color, "Path", o.Request.Path)
	}
	if strings.TrimSpace(o.Request.ContractName) != "" {
		printKV(o.Out, o.Color, "Contract", o.Request.ContractName)
	}
	printKV(o.Out, o.Color, "Target", o.Target)
	_, _ = fmt.Fprintln(o.Out)

	for _, name := range names {
		lc, lok := local[name]
		re, rok := remote[name]
		switch {
		case lok && rok:
			if reflect.DeepEqual(lc.Doc, re.Doc) {
				sum.Unchanged++
				_, _ = fmt.Fprintf(o.Out, "%s %s %s %s\n",
					style.S(o.Color, style.Green, "✓"),
					style.S(o.Color, style.Dim, "Contract:"),
					style.S(o.Color, style.Bold, name),
					style.S(o.Color, style.Green, "[unchanged]"),
				)
				continue
			}
			sum.Changed++
			_, _ = fmt.Fprintf(o.Out, "%s %s %s %s\n",
				style.S(o.Color, style.Yellow, "!"),
				style.S(o.Color, style.Dim, "Contract:"),
				style.S(o.Color, style.Bold, name),
				style.S(o.Color, style.Yellow, "[changed]"),
			)
			ly, err := canonYAMLString(lc.Doc)
			if err != nil {
				return sum, fmt.Errorf("%s: %w", lc.Path, err)
			}
			ry, err := canonYAMLString(re.Doc)
			if err != nil {
				return sum, fmt.Errorf("remote %s: %w", name, err)
			}
			text := strings.TrimSuffix(
				diff.Unified(
					"remote/"+name+".yaml",
					"local/"+filepath.Base(lc.Path),
					ry, ly,
				),
				"\n",
			)
			printIndentedDiff(o.Out, o.Color, text)
			_, _ = fmt.Fprintln(o.Out)
		case lok && !rok:
			sum.OnlyLocal++
			_, _ = fmt.Fprintf(o.Out, "%s %s %s %s\n",
				style.S(o.Color, style.Yellow, "!"),
				style.S(o.Color, style.Dim, "Contract:"),
				style.S(o.Color, style.Bold, name),
				style.S(o.Color, style.Yellow, "[only local, not in export]"),
			)
			_, _ = fmt.Fprintf(o.Out, "    %s\n", style.S(o.Color, style.Dim, lc.Path))
			_, _ = fmt.Fprintln(o.Out)
		case !lok && rok:
			sum.OnlyRemote++
			_, _ = fmt.Fprintf(o.Out, "%s %s %s %s\n",
				style.S(o.Color, style.Yellow, "!"),
				style.S(o.Color, style.Dim, "Contract:"),
				style.S(o.Color, style.Bold, name),
				style.S(o.Color, style.Yellow, "[only remote, missing locally]"),
			)
			_, _ = fmt.Fprintf(o.Out, "    %s %s\n",
				style.S(o.Color, style.Dim, "source_path:"),
				style.S(o.Color, style.Dim, re.File.SourcePath),
			)
			_, _ = fmt.Fprintln(o.Out)
		}
	}

	_, _ = fmt.Fprintln(o.Out)
	_, _ = fmt.Fprintln(o.Out, summaryLine(sum, o.Color))

	if sum.HasWork() {
		_, _ = fmt.Fprintln(o.Out)
		return sum, ErrChanges
	}
	return sum, nil
}

func unionKeys(local map[string]LocalContract, remote map[string]remoteEntry) []string {
	keys := make(map[string]struct{})
	for k := range local {
		keys[k] = struct{}{}
	}
	for k := range remote {
		keys[k] = struct{}{}
	}
	out := make([]string, 0, len(keys))
	for k := range keys {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func printKV(out io.Writer, color bool, k, v string) {
	_, _ = fmt.Fprintf(out, "%s %s\n", style.S(color, style.Dim, k+":"), style.S(color, style.Bold, v))
}

func summaryLine(s Summary, color bool) string {
	if !s.HasWork() {
		n := s.Unchanged
		dot := style.S(color, style.Dim, "·")
		ok := style.S(color, style.Green, "ok")
		if n == 1 {
			return fmt.Sprintf("1 contract %s %s", dot, ok)
		}
		return fmt.Sprintf("%d contracts %s %s", n, dot, ok)
	}
	var parts []string
	if s.Unchanged > 0 {
		parts = append(parts, style.S(color, style.Gray, fmt.Sprintf("%d unchanged", s.Unchanged)))
	}
	if s.Changed > 0 {
		parts = append(parts, style.S(color, style.Yellow, fmt.Sprintf("%d changed", s.Changed)))
	}
	if s.OnlyLocal > 0 {
		parts = append(parts, style.S(color, style.Yellow, fmt.Sprintf("%d only local", s.OnlyLocal)))
	}
	if s.OnlyRemote > 0 {
		parts = append(parts, style.S(color, style.Yellow, fmt.Sprintf("%d only remote", s.OnlyRemote)))
	}
	return strings.Join(parts, " · ")
}

func printIndentedDiff(out io.Writer, color bool, unified string) {
	lines := strings.Split(unified, "\n")
	var beforeFirstMinusMinus bool
	for _, line := range lines {
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "---") && !beforeFirstMinusMinus {
			_, _ = fmt.Fprintln(out)
			beforeFirstMinusMinus = true
		}
		var colored string
		switch {
		case strings.HasPrefix(line, "+++"), strings.HasPrefix(line, "---"), strings.HasPrefix(line, "@@"):
			colored = style.S(color, style.Dim, line)
		case strings.HasPrefix(line, "+"):
			colored = style.S(color, style.Green, line)
		case strings.HasPrefix(line, "-"):
			colored = style.S(color, style.Red, line)
		default:
			colored = style.S(color, style.Dim, line)
		}
		_, _ = fmt.Fprintf(out, "    %s\n", colored)
		if strings.HasPrefix(line, "+++") {
			_, _ = fmt.Fprintln(out)
		}
		if strings.HasPrefix(line, "@@") {
			_, _ = fmt.Fprintln(out)
		}
	}
}

// canonYAMLString renders a document as YAML via JSON round-trip + convertfmt (same pipeline idea as export --format yaml).
func canonYAMLString(doc map[string]any) (string, error) {
	j, err := json.Marshal(doc)
	if err != nil {
		return "", err
	}
	b, err := convertfmt.Convert(j, ".json", "yaml")
	if err != nil {
		return "", err
	}
	return string(b), nil
}
