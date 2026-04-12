// Package outfiles writes exported contract files to disk.
package outfiles

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	apiclient "github.com/merionyx/api-gateway/internal/cli/apiserver"
	"github.com/merionyx/api-gateway/internal/cli/contractfmt"
	"github.com/merionyx/api-gateway/internal/cli/convertfmt"
)

func sanitizeBase(name string) string {
	name = strings.ReplaceAll(name, "/", "_")
	name = strings.ReplaceAll(name, "\\", "_")
	return name
}

// WriteExported writes API response files into outDir.
// If formatFlag is set, converts on disk to yaml or json; otherwise keeps extension from source_path.
func WriteExported(files []apiclient.ExportFile, outDir, formatFlag string) error {
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}
	for _, f := range files {
		raw, err := apiclient.DecodePayload(f)
		if err != nil {
			return fmt.Errorf("file %s: %w", f.ContractName, err)
		}
		sourceExt := strings.ToLower(filepath.Ext(f.SourcePath))
		data := raw
		var outExt string
		if strings.TrimSpace(formatFlag) != "" {
			tgt := strings.ToLower(strings.TrimSpace(formatFlag))
			if tgt == "yml" {
				tgt = "yaml"
			}
			conv, err := contractfmt.FormatBytes(raw, sourceExt, tgt)
			if err != nil {
				return fmt.Errorf("contract %s: format: %w", f.ContractName, err)
			}
			data = conv
			if tgt == "json" {
				outExt = ".json"
			} else {
				outExt = ".yaml"
			}
		} else {
			var err error
			outExt, err = convertfmt.OutputExt(f.SourcePath, "")
			if err != nil {
				return fmt.Errorf("contract %s: %w", f.ContractName, err)
			}
		}
		base := sanitizeBase(f.ContractName) + outExt
		path := filepath.Join(outDir, base)
		if err := os.WriteFile(path, data, 0o644); err != nil {
			return fmt.Errorf("write %s: %w", path, err)
		}
	}
	return nil
}
