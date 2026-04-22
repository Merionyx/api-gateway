package models

import (
	"sort"

	"github.com/merionyx/api-gateway/internal/shared/bundlekey"
)

// CanonicalEnvironmentsForStorage returns a copy with stable ordering so JSON in etcd
// matches across Register and Heartbeat regardless of proto/map iteration order.
func CanonicalEnvironmentsForStorage(envs []EnvironmentInfo) []EnvironmentInfo {
	if len(envs) == 0 {
		return nil
	}
	out := make([]EnvironmentInfo, 0, len(envs))
	for _, e := range envs {
		bunds := append([]BundleInfo(nil), e.Bundles...)
		sort.Slice(bunds, func(i, j int) bool {
			ki := bundlekey.Build(bunds[i].Repository, bunds[i].Ref, bunds[i].Path)
			kj := bundlekey.Build(bunds[j].Repository, bunds[j].Ref, bunds[j].Path)
			if ki != kj {
				return ki < kj
			}
			return bunds[i].Name < bunds[j].Name
		})
		svcs := append([]ServiceInfo(nil), e.Services...)
		sort.Slice(svcs, func(i, j int) bool { return svcs[i].Name < svcs[j].Name })
		out = append(out, EnvironmentInfo{
			Name:                    e.Name,
			Bundles:                 bunds,
			EffectiveGeneration:     e.EffectiveGeneration,
			SourcesFingerprint:      e.SourcesFingerprint,
			EnvironmentConfigSource: e.EnvironmentConfigSource,
			Services:                svcs,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Name < out[j].Name
	})
	return out
}
