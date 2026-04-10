package validate

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

func isContractFile(name string) bool {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".yaml", ".yml", ".json":
		return true
	default:
		return false
	}
}

// CollectFiles returns a list of contract files: if target is a file, a single path;
// if a directory, all nested *.yaml, *.yml, *.json (non-recursive skip of hidden dirs optional — we walk all).
func CollectFiles(target string) ([]string, error) {
	var out []string
	fi, err := os.Stat(filepath.Clean(target))
	if err != nil {
		return nil, err
	}
	if !fi.IsDir() {
		return []string{filepath.Clean(target)}, nil
	}
	err = filepath.WalkDir(target, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		if !isContractFile(d.Name()) {
			return nil
		}
		out = append(out, path)
		return nil
	})
	return out, err
}
