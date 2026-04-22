package envmodel

import (
	"sort"

	"github.com/merionyx/api-gateway/internal/controller/domain/models"
)

// UnionStaticBundles merges two static bundle lists by dedupe key (see BundleKey). Later slices override earlier entries for the same key. Output order is sorted by key.
func UnionStaticBundles(a, b []models.StaticContractBundleConfig) []models.StaticContractBundleConfig {
	byKey := make(map[string]models.StaticContractBundleConfig)
	for _, x := range a {
		byKey[BundleKey(x)] = x
	}
	for _, x := range b {
		byKey[BundleKey(x)] = x
	}
	keys := make([]string, 0, len(byKey))
	for k := range byKey {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make([]models.StaticContractBundleConfig, 0, len(keys))
	for _, k := range keys {
		out = append(out, byKey[k])
	}
	return out
}

// UnionStaticServices merges by service Name; later list overrides the same name. Output is sorted by name.
func UnionStaticServices(a, b []models.StaticServiceConfig) []models.StaticServiceConfig {
	byName := make(map[string]models.StaticServiceConfig)
	for _, s := range a {
		byName[s.Name] = s
	}
	for _, s := range b {
		byName[s.Name] = s
	}
	names := make([]string, 0, len(byName))
	for n := range byName {
		names = append(names, n)
	}
	sort.Strings(names)
	out := make([]models.StaticServiceConfig, 0, len(names))
	for _, n := range names {
		out = append(out, byName[n])
	}
	return out
}
