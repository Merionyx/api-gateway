package usecase

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sort"

	pb "github.com/merionyx/api-gateway/pkg/grpc/controller_registry/v1"
	"google.golang.org/protobuf/proto"

	"github.com/merionyx/api-gateway/internal/controller/domain/models"
	"github.com/merionyx/api-gateway/internal/controller/effective"
	"github.com/merionyx/api-gateway/internal/controller/envmodel"
	"github.com/merionyx/api-gateway/internal/shared/bundlekey"
	sharedgit "github.com/merionyx/api-gateway/internal/shared/git"
)

func (uc *APIServerSyncUseCase) saveSnapshotsToEtcd(ctx context.Context, bundleKey string, snapshots []sharedgit.ContractSnapshot) error {
	repository, ref, bundlePath, err := bundlekey.Parse(bundleKey)
	if err != nil {
		return err
	}

	for _, s := range snapshots {
		cs := sharedToControllerSnapshot(s)
		slog.Info("Saving snapshot to etcd", "repository", repository, "ref", ref, "path", bundlePath, "contract", s.Name)
		if err := uc.schemaRepo.SaveContractSnapshot(ctx, repository, ref, bundlePath, s.Name, cs); err != nil {
			return fmt.Errorf("save snapshot %s: %w", s.Name, err)
		}
	}
	return nil
}

func (uc *APIServerSyncUseCase) updateXDSSnapshot(ctx context.Context, environment string) error {
	slog.Info("Updating xDS snapshot", "environment", environment)
	if uc.reconciler == nil {
		return nil
	}
	// No materialized writes on follower / hot path (leader CRUD and memory rebuild use writeMaterialized).
	return uc.reconciler.ReconcileOne(ctx, environment, false)
}

// effectiveEnvironmentSkeleton uses [effective.MergeMemoryAndControllerEtcd] (ADR 0001) for this name.
func (uc *APIServerSyncUseCase) effectiveEnvironmentSkeleton(ctx context.Context, name string) (*models.Environment, error) {
	var mem *models.Environment
	if m, err := uc.inMemoryEnvironmentsRepo.GetEnvironment(ctx, name); err == nil {
		mem = m
	}

	var etcdEnv *models.Environment
	if uc.environmentRepo != nil {
		if e, err := uc.environmentRepo.GetEnvironment(ctx, name); err == nil {
			etcdEnv = e
		}
	}

	skel, err := effective.MergeMemoryAndControllerEtcd(mem, etcdEnv)
	if err != nil {
		if errors.Is(err, effective.ErrNotFound) {
			return nil, fmt.Errorf("environment %s not found", name)
		}
		return nil, err
	}
	return skel, nil
}

// buildEnvironmentsForAPIServer returns the full declared environment set for Register/Heartbeat
// (union of names from in-memory and etcd CRUD), with merged bundles per name.
func (uc *APIServerSyncUseCase) buildEnvironmentsForAPIServer(ctx context.Context) ([]*pb.EnvironmentInfo, error) {
	names := uc.collectEnvironmentNames(ctx)
	sort.Strings(names)
	out := make([]*pb.EnvironmentInfo, 0, len(names))
	for _, n := range names {
		skel, err := uc.effectiveEnvironmentSkeleton(ctx, n)
		if err != nil {
			slog.Warn("buildEnvironmentsForAPIServer: skip environment", "name", n, "error", err)
			continue
		}
		file, k8s := uc.fileK8sSlices(ctx, n)
		fileS, k8sS := uc.fileK8sServiceSlices(ctx, n)
		inF, inK8 := uc.environmentInMemoryLayers(n)
		var etcdE *models.Environment
		if uc.environmentRepo != nil {
			if e, err := uc.environmentRepo.GetEnvironment(ctx, n); err == nil {
				etcdE = e
			}
		}
		etcdB := etcdStaticBundles(etcdE)
		etcdS := etcdStaticServices(etcdE)
		inEtcd := etcdE != nil
		envLayer := envmodel.ConfigSourceForEnvironmentLayers(inF, inK8, inEtcd)
		fp := envmodel.FingerprintStaticEnvironment(skel)
		em := &pb.EnvironmentMeta{
			Provenance:         provenancePB(envLayer),
			SourcesFingerprint: proto.String(fp),
		}
		if uc.materialized != nil {
			if doc, err := uc.materialized.Get(ctx, n); err != nil {
				slog.Warn("buildEnvironmentsForAPIServer: read materialized generation", "environment", n, "error", err)
			} else if doc != nil {
				g := doc.Generation
				em.EffectiveGeneration = &g
			}
		}
		pbEnv := &pb.EnvironmentInfo{
			Name: skel.Name,
			Meta: em,
		}
		var bundles []*pb.BundleInfo
		if skel.Bundles != nil {
			for _, b := range skel.Bundles.Static {
				src := envmodel.ConfigSourceForStaticBundle(b, file, k8s, etcdB)
				bi := &pb.BundleInfo{
					Name: b.Name, Repository: b.Repository, Ref: b.Ref, Path: b.Path,
				}
				if p := provenancePB(src); p != nil {
					bi.Meta = &pb.BundleMeta{Provenance: p}
				}
				bundles = append(bundles, bi)
			}
		}
		pbEnv.Bundles = bundles
		var services []*pb.ServiceInfo
		if skel.Services != nil {
			for _, s := range skel.Services.Static {
				src := envmodel.ConfigSourceForStaticService(s, fileS, k8sS, etcdS)
				si := &pb.ServiceInfo{Name: s.Name, Upstream: s.Upstream}
				if p := provenancePB(src); p != nil {
					si.Meta = &pb.ServiceMeta{Provenance: p}
				}
				services = append(services, si)
			}
		}
		pbEnv.Services = services
		out = append(out, pbEnv)
	}
	return out, nil
}

func (uc *APIServerSyncUseCase) collectEnvironmentNames(ctx context.Context) []string {
	names := make(map[string]struct{})
	if m, err := uc.inMemoryEnvironmentsRepo.ListEnvironments(ctx); err != nil {
		slog.Warn("collectEnvironmentNames: in-memory list failed", "error", err)
	} else {
		for k := range m {
			names[k] = struct{}{}
		}
	}
	if uc.environmentRepo != nil {
		if m, err := uc.environmentRepo.ListEnvironments(ctx); err != nil {
			slog.Warn("collectEnvironmentNames: etcd environments list failed", "error", err)
		} else {
			for k := range m {
				names[k] = struct{}{}
			}
		}
	}
	out := make([]string, 0, len(names))
	for k := range names {
		out = append(out, k)
	}
	return out
}

func (uc *APIServerSyncUseCase) environmentWithSnapshotsFromSchema(ctx context.Context, src *models.Environment) *models.Environment {
	out := &models.Environment{
		Name:      src.Name,
		Type:      src.Type,
		Bundles:   src.Bundles,
		Services:  src.Services,
		Snapshots: nil,
	}
	for _, bundle := range src.Bundles.Static {
		snaps, err := uc.schemaRepo.ListContractSnapshots(ctx, bundle.Repository, bundle.Ref, bundle.Path)
		if err != nil {
			slog.Warn("ListContractSnapshots failed", "environment", src.Name, "repository", bundle.Repository, "ref", bundle.Ref, "path", bundle.Path, "error", err)
			continue
		}
		out.Snapshots = append(out.Snapshots, snaps...)
	}
	return out
}

func sharedToControllerSnapshot(s sharedgit.ContractSnapshot) *models.ContractSnapshot {
	apps := make([]models.App, len(s.Access.Apps))
	for i, a := range s.Access.Apps {
		apps[i] = models.App{AppID: a.AppID, Environments: a.Environments}
	}
	return &models.ContractSnapshot{
		Name:                  s.Name,
		Prefix:                s.Prefix,
		Upstream:              models.ContractUpstream{Name: s.Upstream.Name},
		AllowUndefinedMethods: s.AllowUndefinedMethods,
		Access: models.Access{
			Secure: s.Access.Secure,
			Apps:   apps,
		},
	}
}
