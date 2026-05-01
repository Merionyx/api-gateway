package contractdiff

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/merionyx/api-gateway/internal/cli/validate"
)

// LocalContract is one contract file on disk indexed by metadata contract name.
type LocalContract struct {
	Path         string
	ContractName string
	Doc          map[string]any
	Ext          string
}

// LoadLocal walks a file or directory (same rules as validate) and loads contract files keyed by x-api-gateway.contract.name.
func LoadLocal(root string) (map[string]LocalContract, error) {
	paths, err := validate.CollectFiles(root)
	if err != nil {
		return nil, err
	}
	out := make(map[string]LocalContract)
	for _, p := range paths {
		root, err := os.OpenRoot(filepath.Dir(p))
		if err != nil {
			return nil, fmt.Errorf("open %s: %w", p, err)
		}
		raw, err := root.ReadFile(filepath.Base(p))
		_ = root.Close()
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", p, err)
		}
		ext := ExtFromPath(p)
		doc, err := ParseDocument(raw, ext)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", p, err)
		}
		cn, err := ContractNameFromDoc(doc)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", p, err)
		}
		if prev, ok := out[cn]; ok {
			return nil, fmt.Errorf("duplicate contract name %q: %s and %s", cn, prev.Path, p)
		}
		out[cn] = LocalContract{Path: p, ContractName: cn, Doc: doc, Ext: ext}
	}
	return out, nil
}
