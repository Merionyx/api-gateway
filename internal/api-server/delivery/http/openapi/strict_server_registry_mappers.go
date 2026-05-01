package openapi

import (
	"encoding/json"
	"time"

	"github.com/merionyx/api-gateway/internal/api-server/domain/models"
	"github.com/merionyx/api-gateway/internal/api-server/gen/apiserver"
	"github.com/merionyx/api-gateway/internal/shared/bundlekey"
	sharedgit "github.com/merionyx/api-gateway/internal/shared/git"
)

func modelJWKSToAPI(in *models.JWKS) (apiserver.Jwks, error) {
	if in == nil {
		return apiserver.Jwks{}, nil
	}
	var out apiserver.Jwks
	b, err := json.Marshal(in)
	if err != nil {
		return out, err
	}
	if err := json.Unmarshal(b, &out); err != nil {
		return out, err
	}
	return out, nil
}

func controllerToAPI(c models.ControllerInfo) apiserver.Controller {
	envs := make([]apiserver.Environment, 0, len(c.Environments))
	for _, e := range c.Environments {
		envs = append(envs, environmentToAPI(e))
	}
	out := apiserver.Controller{
		ControllerId: c.ControllerID,
		Tenant:       c.Tenant,
		Environments: &envs,
	}
	if c.RegistryPayloadVersion > 0 {
		v := c.RegistryPayloadVersion
		out.RegistryPayloadVersion = &v
	}
	return out
}

func environmentToAPI(e models.EnvironmentInfo) apiserver.Environment {
	bundles := make([]apiserver.Bundle, 0, len(e.Bundles))
	for _, b := range e.Bundles {
		bundles = append(bundles, bundleToAPI(b))
	}
	svcs := make([]apiserver.RegistryService, 0, len(e.Services))
	for _, s := range e.Services {
		svcs = append(svcs, staticServiceToAPI(s))
	}
	out := apiserver.Environment{Name: e.Name, Bundles: &bundles, Services: &svcs}
	if m := environmentMetaToAPI(e.Meta); m != nil {
		out.Meta = m
	}
	return out
}

func environmentMetaToAPI(m *models.EnvironmentMeta) *apiserver.EnvironmentMeta {
	if m == nil {
		return nil
	}
	out := &apiserver.EnvironmentMeta{}
	if p := provenanceToAPI(m.Provenance); p != nil {
		out.Provenance = p
	}
	if m.EffectiveGeneration != nil {
		g := *m.EffectiveGeneration
		out.EffectiveGeneration = &g
	}
	if m.SourcesFingerprint != "" {
		s := m.SourcesFingerprint
		out.SourcesFingerprint = &s
	}
	if m.EnvironmentType != "" {
		et := m.EnvironmentType
		out.EnvironmentType = &et
	}
	if m.MaterializedUpdatedAt != "" {
		if t, err := time.Parse(time.RFC3339Nano, m.MaterializedUpdatedAt); err == nil {
			out.MaterializedUpdatedAt = &t
		} else if t, err := time.Parse(time.RFC3339, m.MaterializedUpdatedAt); err == nil {
			out.MaterializedUpdatedAt = &t
		}
	}
	if m.MaterializedSchemaVersion != nil {
		sv := *m.MaterializedSchemaVersion
		out.MaterializedSchemaVersion = &sv
	}
	if m.MaterializedMismatch != nil {
		mm := *m.MaterializedMismatch
		out.MaterializedMismatch = &mm
	}
	if out.Provenance == nil && out.EffectiveGeneration == nil && out.SourcesFingerprint == nil && out.EnvironmentType == nil && out.MaterializedUpdatedAt == nil && out.MaterializedSchemaVersion == nil && out.MaterializedMismatch == nil {
		return nil
	}
	return out
}

func provenanceToAPI(p *models.Provenance) *apiserver.Provenance {
	if p == nil {
		return nil
	}
	var out apiserver.Provenance
	if p.ConfigSource != "" {
		if cs := modelConfigSourceToAPI(p.ConfigSource); cs != nil {
			out.ConfigSource = cs
		}
	}
	if p.LayerDetail != "" {
		d := p.LayerDetail
		out.LayerDetail = &d
	}
	if out.ConfigSource == nil && out.LayerDetail == nil {
		return nil
	}
	return &out
}

func staticServiceToAPI(s models.ServiceInfo) apiserver.RegistryService {
	n, u := s.Name, s.Upstream
	out := apiserver.RegistryService{Name: n, Upstream: u}
	if s.Scope != "" {
		scope := apiserver.ServiceLineScope(s.Scope)
		if scope.Valid() {
			out.Scope = &scope
		} else {
			unspec := apiserver.ServiceLineScopeUnspecified
			out.Scope = &unspec
		}
	}
	if sm := s.Meta; sm != nil {
		if p := provenanceToAPI(sm.Provenance); p != nil || sm.K8sServiceRef != "" {
			m := &apiserver.ServiceMeta{}
			if p != nil {
				m.Provenance = p
			}
			if sm.K8sServiceRef != "" {
				ksr := sm.K8sServiceRef
				m.K8sServiceRef = &ksr
			}
			if m.Provenance != nil || m.K8sServiceRef != nil {
				out.Meta = m
			}
		}
	}
	return out
}

func bundleFromCanonicalKey(bundleKey string) (apiserver.Bundle, error) {
	repo, ref, path, err := bundlekey.Parse(bundleKey)
	if err != nil {
		return apiserver.Bundle{}, err
	}
	k := bundlekey.Build(repo, ref, path)
	repoP, refP, pathP := repo, ref, path
	return apiserver.Bundle{
		Key:        &k,
		Repository: &repoP,
		Ref:        &refP,
		Path:       &pathP,
	}, nil
}

func bundleToAPI(b models.BundleInfo) apiserver.Bundle {
	nm := b.Name
	repo := b.Repository
	ref := b.Ref
	path := b.Path
	k := bundlekey.Build(b.Repository, b.Ref, b.Path)
	out := apiserver.Bundle{
		Key:        &k,
		Name:       &nm,
		Repository: &repo,
		Ref:        &ref,
		Path:       &path,
	}
	if bm := b.Meta; bm != nil {
		if p := provenanceToAPI(bm.Provenance); p != nil || bm.ResolvedRef != "" || bm.LastSyncUTC != "" || bm.SyncError != "" || bm.K8SResourceRef != "" {
			m := &apiserver.BundleMeta{}
			if p != nil {
				m.Provenance = p
			}
			if bm.ResolvedRef != "" {
				r := bm.ResolvedRef
				m.ResolvedRef = &r
			}
			if bm.LastSyncUTC != "" {
				if t, err := time.Parse(time.RFC3339Nano, bm.LastSyncUTC); err == nil {
					m.LastSyncUtc = &t
				} else if t, err := time.Parse(time.RFC3339, bm.LastSyncUTC); err == nil {
					m.LastSyncUtc = &t
				}
			}
			if bm.SyncError != "" {
				se := bm.SyncError
				m.SyncError = &se
			}
			if bm.K8SResourceRef != "" {
				k := bm.K8SResourceRef
				m.K8sResourceRef = &k
			}
			if m.Provenance != nil || m.ResolvedRef != nil || m.LastSyncUtc != nil || m.SyncError != nil || m.K8sResourceRef != nil {
				out.Meta = m
			}
		}
	}
	return out
}

func modelConfigSourceToAPI(s string) *apiserver.ConfigSource {
	if s == "" {
		return nil
	}
	var c apiserver.ConfigSource
	switch s {
	case "file":
		c = apiserver.ConfigSourceFile
	case "kubernetes":
		c = apiserver.ConfigSourceKubernetes
	case "etcd_grpc":
		c = apiserver.ConfigSourceEtcdGrpc
	case "unspecified":
		c = apiserver.ConfigSourceUnspecified
	default:
		c = apiserver.ConfigSource(s)
	}
	return &c
}

func snapshotsToAPI(in []sharedgit.ContractSnapshot) ([]apiserver.ContractSnapshot, error) {
	out := make([]apiserver.ContractSnapshot, len(in))
	for i := range in {
		b, err := json.Marshal(in[i])
		if err != nil {
			return nil, err
		}
		if err := json.Unmarshal(b, &out[i]); err != nil {
			return nil, err
		}
	}
	return out, nil
}

func bundleKeyFromContractBundleParams(
	bk *apiserver.BundleKeyQuery,
	repo *apiserver.BundleRepoQuery,
	ref *apiserver.BundleRefQuery,
	path *apiserver.BundlePathQuery,
) (string, error) {
	var kb, r, f, p string
	if bk != nil {
		kb = string(*bk)
	}
	if repo != nil {
		r = string(*repo)
	}
	if ref != nil {
		f = string(*ref)
	}
	if path != nil {
		p = string(*path)
	}
	return bundlekey.ResolveFromHTTPQuery(kb, r, f, p)
}
