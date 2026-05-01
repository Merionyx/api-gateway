// Package resource maps CLI resource names (plural, kubectl-style) and aliases to list kinds.
package resource

import "strings"

// Kind identifies a listable API Server resource.
type Kind string

const (
	Controllers   Kind = "controllers"
	Tenants       Kind = "tenants"
	Environments  Kind = "environments"
	Bundles       Kind = "bundles"
	BundleKeys    Kind = "bundle-keys"
	ContractNames Kind = "contract-names"
)

// Entry describes one resource kind for `agwctl list`.
type Entry struct {
	Kind              Kind
	Canonical         string   // plural, hyphenated (shown in help)
	Aliases           []string // lowercase
	RequiresTenant    bool
	RequiresBundleKey bool
}

// Registry is the supported list resources (excludes signing keys, status, etc. by design).
var Registry = []Entry{
	{Kind: Controllers, Canonical: "controllers", Aliases: []string{"controller", "ctrl", "cnt"}},
	{Kind: Tenants, Canonical: "tenants", Aliases: []string{"tenant", "tns", "tn"}},
	{Kind: Environments, Canonical: "environments", Aliases: []string{"environment", "env", "envs"}, RequiresTenant: true},
	{Kind: Bundles, Canonical: "bundles", Aliases: []string{"bundle", "static-bundles", "tenant-bundles", "tbundles", "tbundle"}, RequiresTenant: true},
	{Kind: BundleKeys, Canonical: "bundle-keys", Aliases: []string{"bundlekeys", "bkeys", "bk"}},
	{Kind: ContractNames, Canonical: "contract-names", Aliases: []string{"contracts", "cnames", "cn"}, RequiresBundleKey: true},
}

// Resolve matches user input (case-insensitive) to a registry entry.
func Resolve(name string) (Entry, bool) {
	n := strings.ToLower(strings.TrimSpace(name))
	for _, e := range Registry {
		if n == e.Canonical {
			return e, true
		}
		for _, a := range e.Aliases {
			if n == strings.ToLower(a) {
				return e, true
			}
		}
	}
	return Entry{}, false
}

// CanonicalNames returns comma-separated canonical names for help.
func CanonicalNames() string {
	var b strings.Builder
	for i, e := range Registry {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(e.Canonical)
	}
	return b.String()
}
