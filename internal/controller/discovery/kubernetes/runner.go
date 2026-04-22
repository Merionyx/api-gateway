package kubernetes

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"time"

	gwv1alpha1 "github.com/merionyx/api-gateway/pkg/apis/crd/v1alpha1"

	"github.com/merionyx/api-gateway/internal/controller/config"
	"github.com/merionyx/api-gateway/internal/controller/domain/interfaces"
	"github.com/merionyx/api-gateway/internal/controller/domain/models"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	cgscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	AnnEnvironmentID   = "gateway.merionyx.com/environment-id"
	AnnServiceName     = "gateway.merionyx.com/service-name"
	AnnUpstream        = "gateway.merionyx.com/upstream"
	AnnPort            = "gateway.merionyx.com/port"
	LabelEnvironmentID = "gateway.merionyx.com/environment-id"
)

// Runner periodically lists Kubernetes resources and pushes a snapshot into in-memory repositories.
type Runner struct {
	client  client.Client
	cfg     *config.KubernetesDiscoveryConfig
	envRepo interfaces.InMemoryEnvironmentsRepository
	svcRepo interfaces.InMemoryServiceRepository
}

func restConfig() (*rest.Config, error) {
	rc, err := rest.InClusterConfig()
	if err == nil {
		return rc, nil
	}
	home, herr := os.UserHomeDir()
	if herr != nil {
		return nil, fmt.Errorf("kubeconfig: %w", err)
	}
	return clientcmd.BuildConfigFromFlags("", filepath.Join(home, ".kube", "config"))
}

// NewRunner builds a client from in-cluster or ~/.kube/config.
func NewRunner(kd *config.KubernetesDiscoveryConfig, envRepo interfaces.InMemoryEnvironmentsRepository, svcRepo interfaces.InMemoryServiceRepository) (*Runner, error) {
	rc, err := restConfig()
	if err != nil {
		return nil, err
	}
	sch := runtime.NewScheme()
	utilruntime.Must(cgscheme.AddToScheme(sch))
	utilruntime.Must(gwv1alpha1.AddToScheme(sch))
	cl, err := client.New(rc, client.Options{Scheme: sch})
	if err != nil {
		return nil, err
	}
	return &Runner{client: cl, cfg: kd, envRepo: envRepo, svcRepo: svcRepo}, nil
}

// Run resyncs until ctx is cancelled.
func (r *Runner) Run(ctx context.Context, interval time.Duration) {
	t := time.NewTicker(interval)
	defer t.Stop()
	if err := r.syncOnce(ctx); err != nil {
		slog.Error("kubernetes discovery: initial sync failed", "error", err)
	}
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if err := r.syncOnce(ctx); err != nil {
				slog.Error("kubernetes discovery: sync failed", "error", err)
			}
		}
	}
}

// k8sSyncState is one discovery pass: per-environment view plus controller-global services from K8s.
type k8sSyncState struct {
	envs    map[string]*models.Environment
	globals []models.StaticServiceConfig
}

func (r *Runner) syncOnce(ctx context.Context) error {
	nsList, err := r.allowedNamespaces(ctx)
	if err != nil {
		return err
	}
	listOpts := r.listOpts()
	st := k8sSyncState{envs: make(map[string]*models.Environment)}
	if err := r.loadEnvironmentMapFromCRDs(ctx, nsList, listOpts, &st); err != nil {
		return err
	}
	if err := r.collectContractBundles(ctx, listOpts, &st); err != nil {
		return err
	}
	if err := r.applyGatewayUpstreams(ctx, listOpts, &st); err != nil {
		return err
	}
	if err := r.applyCoreServices(ctx, listOpts, &st); err != nil {
		return err
	}
	return r.reconcileGlobalServices(ctx, &st)
}

func (r *Runner) loadEnvironmentMapFromCRDs(ctx context.Context, nsList []string, listOpts []client.ListOption, st *k8sSyncState) error {
	for _, ns := range nsList {
		var elist gwv1alpha1.EnvironmentList
		if err := r.client.List(ctx, &elist, append(listOpts, client.InNamespace(ns))...); err != nil {
			return fmt.Errorf("list environments in %s: %w", ns, err)
		}
		for i := range elist.Items {
			e := &elist.Items[i]
			id := environmentLogicalName(e)
			st.envs[id] = newEnvModel(id)
		}
	}
	return nil
}

func (r *Runner) collectContractBundles(ctx context.Context, listOpts []client.ListOption, st *k8sSyncState) error {
	var blist gwv1alpha1.ContractBundleList
	if err := r.client.List(ctx, &blist, listOpts...); err != nil {
		return fmt.Errorf("list contractbundles: %w", err)
	}
	for i := range blist.Items {
		b := &blist.Items[i]
		id, err := r.bundleEnvID(ctx, b)
		if err != nil {
			slog.Warn("skip contractbundle", "ns", b.Namespace, "name", b.Name, "error", err)
			continue
		}
		if st.envs[id] == nil {
			st.envs[id] = newEnvModel(id)
		}
		disc := b.Namespace + "/" + b.Name
		st.envs[id].Bundles.Static = append(st.envs[id].Bundles.Static, models.StaticContractBundleConfig{
			Name:         b.Spec.Name,
			Repository:   b.Spec.Repository,
			Ref:          b.Spec.Ref,
			Path:         b.Spec.Path,
			DiscoveryRef: disc,
		})
	}
	return nil
}

func (r *Runner) applyGatewayUpstreams(ctx context.Context, listOpts []client.ListOption, st *k8sSyncState) error {
	var ulist gwv1alpha1.GatewayUpstreamList
	if err := r.client.List(ctx, &ulist, listOpts...); err != nil {
		return fmt.Errorf("list gatewayupstreams: %w", err)
	}
	for i := range ulist.Items {
		u := &ulist.Items[i]
		ug := u.Namespace + "/" + u.Name
		uref := "GatewayUpstream:" + ug
		if u.Spec.EnvironmentID == "" {
			st.globals = append(st.globals, models.StaticServiceConfig{
				Name:         u.Spec.Name,
				Upstream:     u.Spec.Upstream,
				DiscoveryRef: uref,
			})
			continue
		}
		id := u.Spec.EnvironmentID
		if st.envs[id] == nil {
			st.envs[id] = newEnvModel(id)
		}
		st.envs[id].Services.Static = append(st.envs[id].Services.Static, models.StaticServiceConfig{
			Name:         u.Spec.Name,
			Upstream:     u.Spec.Upstream,
			DiscoveryRef: uref,
		})
	}
	return nil
}

func (r *Runner) applyCoreServices(ctx context.Context, listOpts []client.ListOption, st *k8sSyncState) error {
	var slist corev1.ServiceList
	if err := r.client.List(ctx, &slist, listOpts...); err != nil {
		return fmt.Errorf("list services: %w", err)
	}
	for i := range slist.Items {
		svc := &slist.Items[i]
		envID := svc.Annotations[AnnEnvironmentID]
		if envID == "" {
			continue
		}
		svcName := svc.Annotations[AnnServiceName]
		if svcName == "" {
			svcName = svc.Name
		}
		up := svc.Annotations[AnnUpstream]
		if up == "" {
			up = upstreamFromService(svc)
		}
		if st.envs[envID] == nil {
			st.envs[envID] = newEnvModel(envID)
		}
		sref := "core/Service:" + svc.Namespace + "/" + svc.Name
		st.envs[envID].Services.Static = append(st.envs[envID].Services.Static, models.StaticServiceConfig{
			Name:         svcName,
			Upstream:     up,
			DiscoveryRef: sref,
		})
	}
	return nil
}

func (r *Runner) reconcileGlobalServices(ctx context.Context, st *k8sSyncState) error {
	r.svcRepo.SetKubernetesGlobalServices(st.globals)
	return r.envRepo.ApplyKubernetesEnvironments(ctx, st.envs)
}

func (r *Runner) listOpts() []client.ListOption {
	if len(r.cfg.ResourceLabelSelector) == 0 {
		return nil
	}
	return []client.ListOption{client.MatchingLabels(r.cfg.ResourceLabelSelector)}
}

func (r *Runner) allowedNamespaces(ctx context.Context) ([]string, error) {
	var list corev1.NamespaceList
	if err := r.client.List(ctx, &list); err != nil {
		return nil, err
	}
	var out []string
	for i := range list.Items {
		ns := &list.Items[i]
		if len(r.cfg.NamespaceLabelSelector) > 0 && !labelsMatch(r.cfg.NamespaceLabelSelector, ns.Labels) {
			continue
		}
		if len(r.cfg.WatchNamespaces) > 0 && !slices.Contains(r.cfg.WatchNamespaces, ns.Name) {
			continue
		}
		out = append(out, ns.Name)
	}
	return out, nil
}

func labelsMatch(sel, labels map[string]string) bool {
	for k, want := range sel {
		if labels[k] != want {
			return false
		}
	}
	return true
}

func environmentLogicalName(e *gwv1alpha1.Environment) string {
	if e.Spec.LogicalName != "" {
		return e.Spec.LogicalName
	}
	return e.Namespace + "-" + e.Name
}

func newEnvModel(name string) *models.Environment {
	return &models.Environment{
		Name:      name,
		Type:      "kubernetes",
		Bundles:   &models.EnvironmentBundleConfig{Static: []models.StaticContractBundleConfig{}},
		Services:  &models.EnvironmentServiceConfig{Static: []models.StaticServiceConfig{}},
		Snapshots: []models.ContractSnapshot{},
	}
}

func (r *Runner) bundleEnvID(ctx context.Context, b *gwv1alpha1.ContractBundle) (string, error) {
	if b.Spec.EnvironmentID != "" {
		return b.Spec.EnvironmentID, nil
	}
	if id := b.Labels[LabelEnvironmentID]; id != "" {
		return id, nil
	}
	ref := b.Spec.EnvironmentRef
	if ref == nil || ref.Name == "" {
		return "", fmt.Errorf("missing environmentRef/environmentId/label")
	}
	ns := ref.Namespace
	if ns == "" {
		ns = b.Namespace
	}
	var e gwv1alpha1.Environment
	key := client.ObjectKey{Namespace: ns, Name: ref.Name}
	if err := r.client.Get(ctx, key, &e); err != nil {
		return "", err
	}
	return environmentLogicalName(&e), nil
}

func upstreamFromService(svc *corev1.Service) string {
	port := int32(80)
	if p := svc.Annotations[AnnPort]; p != "" {
		if n, err := strconv.ParseInt(p, 10, 32); err == nil && n > 0 {
			port = int32(n)
		}
	} else if len(svc.Spec.Ports) > 0 {
		port = svc.Spec.Ports[0].Port
	}
	return fmt.Sprintf("http://%s.%s.svc.cluster.local:%d", svc.Name, svc.Namespace, port)
}
