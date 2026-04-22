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
		for i := range bunds {
			if m := bunds[i].Meta; m != nil {
				cp := *m
				if p := m.Provenance; p != nil {
					pp := *p
					cp.Provenance = &pp
				}
				bunds[i].Meta = &cp
			}
		}
		sort.Slice(bunds, func(i, j int) bool {
			ki := bundlekey.Build(bunds[i].Repository, bunds[i].Ref, bunds[i].Path)
			kj := bundlekey.Build(bunds[j].Repository, bunds[j].Ref, bunds[j].Path)
			if ki != kj {
				return ki < kj
			}
			return bunds[i].Name < bunds[j].Name
		})
		svcs := append([]ServiceInfo(nil), e.Services...)
		for i := range svcs {
			if m := svcs[i].Meta; m != nil {
				cp := *m
				if p := m.Provenance; p != nil {
					pp := *p
					cp.Provenance = &pp
				}
				svcs[i].Meta = &cp
			}
		}
		sort.Slice(svcs, func(i, j int) bool { return svcs[i].Name < svcs[j].Name })
		var em *EnvironmentMeta
		if e.Meta != nil {
			em = &EnvironmentMeta{SourcesFingerprint: e.Meta.SourcesFingerprint}
			if p := e.Meta.Provenance; p != nil {
				pp := *p
				em.Provenance = &pp
			}
			if e.Meta.EffectiveGeneration != nil {
				g := *e.Meta.EffectiveGeneration
				em.EffectiveGeneration = &g
			}
		}
		if em != nil && em.Provenance == nil && em.EffectiveGeneration == nil && em.SourcesFingerprint == "" {
			em = nil
		}
		out = append(out, EnvironmentInfo{
			Name:     e.Name,
			Bundles:  bunds,
			Services: svcs,
			Meta:     em,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Name < out[j].Name
	})
	return out
}
