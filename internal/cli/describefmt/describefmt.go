// Package describefmt renders nested maps/lists for terminal output:
// two spaces per nesting level, keys dim, scalar values default color, no column padding.
package describefmt

import (
	"fmt"
	"io"
	"sort"
	"strconv"

	"github.com/merionyx/api-gateway/internal/cli/style"
)

const indentUnit = "  "

// Write renders v (typically map[string]any from JSON/YAML) to w.
func Write(w io.Writer, v any, color bool) error {
	norm := Normalize(v)
	render(w, norm, "", color)
	return nil
}

// Normalize converts YAML driver types (e.g. map[any]any) into map[string]any / []any trees.
func Normalize(v any) any {
	switch t := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(t))
		for k, val := range t {
			out[k] = Normalize(val)
		}
		return out
	case map[any]any:
		out := make(map[string]any, len(t))
		for k, val := range t {
			out[fmt.Sprint(k)] = Normalize(val)
		}
		return out
	case []any:
		out := make([]any, len(t))
		for i := range t {
			out[i] = Normalize(t[i])
		}
		return out
	default:
		return v
	}
}

func render(w io.Writer, v any, indent string, color bool) {
	switch t := v.(type) {
	case map[string]any:
		renderMap(w, t, indent, color)
	case []any:
		renderSlice(w, t, indent, color)
	default:
		_, _ = fmt.Fprintln(w, indent+scalarString(t))
	}
}

func renderMap(w io.Writer, m map[string]any, indent string, color bool) {
	if len(m) == 0 {
		_, _ = fmt.Fprintln(w, indent+"{}")
		return
	}
	for _, k := range sortedKeys(m) {
		writeMapEntry(w, indent, k, m[k], color)
	}
}

func writeMapEntry(w io.Writer, indent, k string, val any, color bool) {
	if isScalar(val) {
		_, _ = fmt.Fprintln(w, indent+style.S(color, style.Cyan, k+":")+" "+scalarString(val))
		return
	}
	_, _ = fmt.Fprintln(w, indent+style.S(color, style.Cyan, k+":"))
	render(w, val, indent+indentUnit, color)
}

func renderSlice(w io.Writer, items []any, indent string, color bool) {
	if len(items) == 0 {
		_, _ = fmt.Fprintln(w, indent+"[]")
		return
	}
	for _, el := range items {
		switch t := el.(type) {
		case map[string]any:
			renderListItemMap(w, t, indent, color)
		case []any:
			_, _ = fmt.Fprintln(w, indent+"-")
			renderSlice(w, t, indent+indentUnit, color)
		default:
			_, _ = fmt.Fprintln(w, indent+"- "+scalarString(t))
		}
	}
}

// renderListItemMap prints one YAML list item that is a map: optional "- k: v" merge for the first
// scalar pair, then remaining keys at the list-item body indent.
func renderListItemMap(w io.Writer, m map[string]any, lineIndent string, color bool) {
	keys := sortedKeys(m)
	if len(keys) == 0 {
		_, _ = fmt.Fprintln(w, lineIndent+"- {}")
		return
	}
	bodyIndent := lineIndent + indentUnit // text column after "- "

	first, rest := keys[0], keys[1:]
	v0 := m[first]

	if isScalar(v0) {
		_, _ = fmt.Fprintln(w, lineIndent+"- "+style.S(color, style.Cyan, first+":")+" "+scalarString(v0))
		for _, k := range rest {
			writeMapEntry(w, bodyIndent, k, m[k], color)
		}
		return
	}

	_, _ = fmt.Fprintln(w, lineIndent+"- "+style.S(color, style.Cyan, first+":"))
	render(w, v0, bodyIndent+indentUnit, color)
	for _, k := range rest {
		writeMapEntry(w, bodyIndent, k, m[k], color)
	}
}

func sortedKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func isScalar(v any) bool {
	switch v.(type) {
	case map[string]any, map[any]any, []any:
		return false
	default:
		return true
	}
}

func scalarString(v any) string {
	switch t := v.(type) {
	case nil:
		return "null"
	case string:
		return t
	case bool:
		return strconv.FormatBool(t)
	case int:
		return strconv.FormatInt(int64(t), 10)
	case int64:
		return strconv.FormatInt(t, 10)
	case float64:
		return strconv.FormatFloat(t, 'g', -1, 64)
	case float32:
		return strconv.FormatFloat(float64(t), 'g', -1, 32)
	default:
		return fmt.Sprint(t)
	}
}
