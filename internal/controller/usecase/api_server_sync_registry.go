package usecase

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sort"

	pb "github.com/merionyx/api-gateway/pkg/grpc/controller_registry/v1"
	"google.golang.org/protobuf/proto"

	"github.com/merionyx/api-gateway/internal/controller/domain/interfaces"
	"github.com/merionyx/api-gateway/internal/controller/domain/models"
	"github.com/merionyx/api-gateway/internal/controller/effective"
	"github.com/merionyx/api-gateway/internal/controller/envmodel"
	"github.com/merionyx/api-gateway/internal/controller/portservices"
	ctrlrepoetcd "github.com/merionyx/api-gateway/internal/controller/repository/etcd"
	sharedgit "github.com/merionyx/api-gateway/internal/shared/git"
)

// registryEnvironmentsBuilder assembles controller_registry proto environments (static merge,
// provenance, materialized generation) for Register/Heartbeat. It is independent of the gRPC stream.
type registryEnvironmentsBuilder struct {
	inMemoryEnvironmentsRepo interfaces.InMemoryEnvironmentsRepository
	rootServicePool          interfaces.InMemoryServiceRepository
	environmentRepo          interfaces.EnvironmentRepository
	materialized             *ctrlrepoetcd.MaterializedStore
	schemaRepo               interfaces.SchemaRepository
}

func newRegistryEnvironmentsBuilder(
	inMem interfaces.InMemoryEnvironmentsRepository,
	rootSvcPool interfaces.InMemoryServiceRepository,
	environmentRepo interfaces.EnvironmentRepository,
	materialized *ctrlrepoetcd.MaterializedStore,
	schema interfaces.SchemaRepository,
) *registryEnvironmentsBuilder {
	return &registryEnvironmentsBuilder{
		inMemoryEnvironmentsRepo: inMem,
		rootServicePool:          rootSvcPool,
		environmentRepo:          environmentRepo,
		materialized:             materialized,
		schemaRepo:               schema,
	}
}

// loadMergedEnvironment loads the in-memory layer and the controller-etcd copy for name, merges
// with [effective.MergeMemoryAndControllerEtcd] (ADR 0001), and returns the raw etcd environment
// (if any) for provenance slices. A single Get per name avoids duplicate etcd I/O in the build loop.
func (b *registryEnvironmentsBuilder) loadMergedEnvironment(ctx context.Context, name string) (skel *models.Environment, etcd *models.Environment, err error) {
	var mem *models.Environment
	if m, err2 := b.inMemoryEnvironmentsRepo.GetEnvironment(ctx, name); err2 == nil {
		mem = m
	}
	if b.environmentRepo != nil {
		if e, err2 := b.environmentRepo.GetEnvironment(ctx, name); err2 == nil {
			etcd = e
		}
	}
	skel, err = effective.MergeMemoryAndControllerEtcd(mem, etcd)
	if err != nil {
		if errors.Is(err, effective.ErrNotFound) {
			return nil, nil, fmt.Errorf("environment %s not found", name)
		}
		return nil, nil, err
	}
	return skel, etcd, nil
}

// buildEnvironmentsForAPIServer — full payload Register/Heartbeat (union of in-memory + etcd names) with provenance
// per env. RegistryEnvironmentsBuildReport lists degradations; empty payload + no Warnings — «normal empty»,
// otherwise Warnings. Cancellation ctx — error upwards.
func (b *registryEnvironmentsBuilder) buildEnvironmentsForAPIServer(ctx context.Context) ([]*pb.EnvironmentInfo, RegistryEnvironmentsBuildReport, error) {
	if err := ctx.Err(); err != nil {
		return nil, RegistryEnvironmentsBuildReport{}, err
	}
	var report RegistryEnvironmentsBuildReport
	names, nameList := b.collectEnvironmentNames(ctx)
	sort.Strings(names)
	report.appendNameListWarnings(nameList)
	out := make([]*pb.EnvironmentInfo, 0, len(names))
	for _, n := range names {
		if err := ctx.Err(); err != nil {
			return out, report, err
		}
		skel, etcd, err := b.loadMergedEnvironment(ctx, n)
		if err != nil {
			report.addWarning(RegistryBuildWarningEnvMerge, n, err)
			continue
		}
		out = append(out, b.buildOneEnvironmentForAPIServer(ctx, n, skel, etcd, &report))
	}
	return out, report, nil
}

// buildOneEnvironmentForAPIServer assembles a single [pb.EnvironmentInfo] with provenance and optional root pool.
func (b *registryEnvironmentsBuilder) buildOneEnvironmentForAPIServer(
	ctx context.Context,
	name string,
	skel *models.Environment,
	etcdE *models.Environment,
	report *RegistryEnvironmentsBuildReport,
) *pb.EnvironmentInfo {
	file, k8s := b.fileK8sSlices(ctx, name)
	fileS, k8sS := b.fileK8sServiceSlices(ctx, name)
	inF, inK8 := b.environmentInMemoryLayers(name)
	etcdB := etcdStaticBundles(etcdE)
	etcdS := etcdStaticServices(etcdE)
	inEtcd := etcdE != nil
	envLayer := envmodel.ConfigSourceForEnvironmentLayers(inF, inK8, inEtcd)
	fp := envmodel.FingerprintStaticEnvironment(skel)
	em := &pb.EnvironmentMeta{
		Provenance:         provenanceWithLayer(envLayer, environmentDominantLayerDetail(inF, inK8, inEtcd)),
		SourcesFingerprint: proto.String(fp),
	}
	if skel.Type != "" {
		em.EnvironmentType = proto.String(skel.Type)
	}
	if b.materialized != nil {
		if doc, err := b.materialized.Get(ctx, name); err != nil {
			report.addWarning(RegistryBuildWarningMaterializedGet, name, err)
		} else if doc != nil {
			g := doc.Generation
			em.EffectiveGeneration = &g
			em.MaterializedUpdatedAt = proto.String(doc.UpdatedAt)
			if doc.SchemaVersion < 0 || doc.SchemaVersion > math.MaxInt32 {
				report.addWarning(
					RegistryBuildWarningMaterializedGet,
					name,
					fmt.Errorf("schema version out of int32 range: %d", doc.SchemaVersion),
				)
			} else {
				sv := int32(doc.SchemaVersion)
				em.MaterializedSchemaVersion = &sv
			}
			if doc.SourcesFingerprint != "" && doc.SourcesFingerprint != fp {
				m := true
				em.MaterializedMismatch = &m
			}
		}
	}
	pbEnv := &pb.EnvironmentInfo{
		Name: skel.Name,
		Meta: em,
	}
	pbEnv.Bundles = b.buildBundleInfosForRegistry(skel, file, k8s, etcdB)
	pbEnv.Services = b.buildServiceInfosForRegistry(skel, fileS, k8sS, etcdS)
	return pbEnv
}

func (b *registryEnvironmentsBuilder) buildBundleInfosForRegistry(
	skel *models.Environment,
	file, k8s, etcdB []models.StaticContractBundleConfig,
) []*pb.BundleInfo {
	if skel == nil || skel.Bundles == nil {
		return nil
	}
	bundles := make([]*pb.BundleInfo, 0, len(skel.Bundles.Static))
	for _, bndl := range skel.Bundles.Static {
		src := envmodel.ConfigSourceForStaticBundle(bndl, file, k8s, etcdB)
		bi := &pb.BundleInfo{
			Name: bndl.Name, Repository: bndl.Repository, Ref: bndl.Ref, Path: bndl.Path,
		}
		if p := provenanceWithLayer(src, staticBundleProvenanceLayer(src, bndl.DiscoveryRef)); p != nil {
			bm := &pb.BundleMeta{Provenance: p}
			if bndl.DiscoveryRef != "" {
				bm.K8SResourceRef = proto.String(bndl.DiscoveryRef)
			}
			bi.Meta = bm
		} else if bndl.DiscoveryRef != "" {
			bi.Meta = &pb.BundleMeta{K8SResourceRef: proto.String(bndl.DiscoveryRef)}
		}
		bundles = append(bundles, bi)
	}
	return bundles
}

func (b *registryEnvironmentsBuilder) buildServiceInfosForRegistry(
	skel *models.Environment,
	fileS, k8sS, etcdS []models.StaticServiceConfig,
) []*pb.ServiceInfo {
	seenService := make(map[string]struct{})
	var services []*pb.ServiceInfo
	if skel != nil && skel.Services != nil {
		for _, s := range skel.Services.Static {
			seenService[s.Name] = struct{}{}
			src := envmodel.ConfigSourceForStaticService(s, fileS, k8sS, etcdS)
			si := &pb.ServiceInfo{
				Name:     s.Name,
				Upstream: s.Upstream,
				Scope:    ptrServiceScope(pb.ServiceLineScope_SERVICE_LINE_SCOPE_ENVIRONMENT),
			}
			layer := staticServiceProvenanceLayer(src, s.DiscoveryRef)
			if p := provenanceWithLayer(src, layer); p != nil {
				sm := &pb.ServiceMeta{Provenance: p}
				if s.DiscoveryRef != "" {
					sm.K8SServiceRef = proto.String(s.DiscoveryRef)
				}
				si.Meta = sm
			} else if s.DiscoveryRef != "" {
				si.Meta = &pb.ServiceMeta{K8SServiceRef: proto.String(s.DiscoveryRef)}
			}
			services = append(services, si)
		}
	}
	// Controller root pool: [portservices.RootPoolDeduplicatedExcludingNames] + same upstream merge as xDS
	// [portservices.MergeEnvStaticWithRootPoolUpstreams].
	if b.rootServicePool != nil {
		f, k := portservices.RootPoolDeduplicatedExcludingNames(b.rootServicePool, seenService)
		for _, s := range f {
			seenService[s.Name] = struct{}{}
			si := &pb.ServiceInfo{
				Name:     s.Name,
				Upstream: s.Upstream,
				Scope:    ptrServiceScope(pb.ServiceLineScope_SERVICE_LINE_SCOPE_CONTROLLER_ROOT),
			}
			if p := provenanceWithLayer(envmodel.StaticConfigFile, "pool:controller_root:file"); p != nil {
				sm := &pb.ServiceMeta{Provenance: p}
				if s.DiscoveryRef != "" {
					sm.K8SServiceRef = proto.String(s.DiscoveryRef)
				}
				si.Meta = sm
			} else if s.DiscoveryRef != "" {
				si.Meta = &pb.ServiceMeta{K8SServiceRef: proto.String(s.DiscoveryRef)}
			}
			services = append(services, si)
		}
		for _, s := range k {
			seenService[s.Name] = struct{}{}
			si := &pb.ServiceInfo{
				Name:     s.Name,
				Upstream: s.Upstream,
				Scope:    ptrServiceScope(pb.ServiceLineScope_SERVICE_LINE_SCOPE_CONTROLLER_ROOT),
			}
			if p := provenanceWithLayer(envmodel.StaticConfigKubernetes, "pool:controller_root:kubernetes"); p != nil {
				sm := &pb.ServiceMeta{Provenance: p}
				if s.DiscoveryRef != "" {
					sm.K8SServiceRef = proto.String(s.DiscoveryRef)
				}
				si.Meta = sm
			} else if s.DiscoveryRef != "" {
				si.Meta = &pb.ServiceMeta{K8SServiceRef: proto.String(s.DiscoveryRef)}
			}
			services = append(services, si)
		}
	}
	sort.Slice(services, func(i, j int) bool { return services[i].Name < services[j].Name })
	return services
}

// collectEnvironmentNames — union of names from two sources; failure of one — warnings + data from the other.
func (b *registryEnvironmentsBuilder) collectEnvironmentNames(ctx context.Context) ([]string, []RegistryEnvironmentsBuildWarning) {
	names := make(map[string]struct{})
	var warns []RegistryEnvironmentsBuildWarning
	if m, err := b.inMemoryEnvironmentsRepo.ListEnvironments(ctx); err != nil {
		warns = append(warns, RegistryEnvironmentsBuildWarning{
			Kind: RegistryBuildWarningInMemoryList, Subject: "in_memory", Err: err,
		})
	} else {
		for k := range m {
			names[k] = struct{}{}
		}
	}
	if b.environmentRepo != nil {
		if m, err := b.environmentRepo.ListEnvironments(ctx); err != nil {
			warns = append(warns, RegistryEnvironmentsBuildWarning{
				Kind: RegistryBuildWarningEtcdList, Subject: "etcd", Err: err,
			})
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
	return out, warns
}

// environmentWithSnapshotsFromSchema is unused in the hot path but kept for symmetry; second value — degradations by snapshots.
func (b *registryEnvironmentsBuilder) environmentWithSnapshotsFromSchema(ctx context.Context, src *models.Environment) (*models.Environment, []RegistryEnvironmentsBuildWarning) {
	var warns []RegistryEnvironmentsBuildWarning
	out := &models.Environment{
		Name:      src.Name,
		Type:      src.Type,
		Bundles:   src.Bundles,
		Services:  src.Services,
		Snapshots: nil,
	}
	if src.Bundles == nil {
		return out, nil
	}
	for _, bundle := range src.Bundles.Static {
		snaps, err := b.schemaRepo.ListContractSnapshots(ctx, bundle.Repository, bundle.Ref, bundle.Path)
		if err != nil {
			warns = append(warns, RegistryEnvironmentsBuildWarning{
				Kind:    RegistryBuildWarningListContractSnapshots,
				Subject: src.Name + "/" + bundle.Repository,
				Err:     err,
			})
			continue
		}
		out.Snapshots = append(out.Snapshots, snaps...)
	}
	if len(warns) == 0 {
		return out, nil
	}
	return out, warns
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
