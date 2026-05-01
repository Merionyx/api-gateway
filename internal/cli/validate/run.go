package validate

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
)

// Options controls validation behaviour.
type Options struct {
	CheckContent bool
}

// FileResult is one file path and all issues found (may be empty if valid).
type FileResult struct {
	Path    string
	Issues  []string
	Skipped bool // true if file could not be read or parsed (still reported)
}

// Run validates every contract file under target (file or directory). Errors in one file never stop others.
func Run(ctx context.Context, target string, opts Options) []FileResult {
	paths, err := CollectFiles(target)
	if err != nil {
		return []FileResult{{Path: target, Issues: []string{fmt.Sprintf("[path] %v", err)}, Skipped: true}}
	}
	if len(paths) == 0 {
		return []FileResult{{Path: target, Issues: []string{"[path] no .yaml/.yml/.json files found"}, Skipped: true}}
	}

	var results []FileResult
	for _, p := range paths {
		res := FileResult{Path: p}
		dirRoot, rerr := os.OpenRoot(filepath.Dir(p))
		if rerr != nil {
			res.Issues = append(res.Issues, fmt.Sprintf("[read] %v", rerr))
			res.Skipped = true
			results = append(results, res)
			continue
		}
		data, rerr := dirRoot.ReadFile(filepath.Base(p))
		_ = dirRoot.Close()
		if rerr != nil {
			res.Issues = append(res.Issues, fmt.Sprintf("[read] %v", rerr))
			res.Skipped = true
			results = append(results, res)
			continue
		}
		doc, perr := ParseRoot(data)
		if perr != nil {
			res.Issues = append(res.Issues, fmt.Sprintf("[parse] %v", perr))
			res.Skipped = true
			results = append(results, res)
			continue
		}
		res.Issues = append(res.Issues, ValidateXApiGateway(doc)...)
		if opts.CheckContent {
			res.Issues = append(res.Issues, RunContentChecks(ctx, p, data, doc)...)
		}
		results = append(results, res)
	}
	return results
}
