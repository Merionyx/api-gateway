package envmodel

import "github.com/merionyx/api-gateway/internal/controller/domain/models"

// BundleKey returns a stable string key for repository/ref/path/name.
func BundleKey(b models.StaticContractBundleConfig) string {
	return b.Repository + "\x00" + b.Ref + "\x00" + b.Path + "\x00" + b.Name
}
