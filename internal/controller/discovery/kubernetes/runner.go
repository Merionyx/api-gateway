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

	"merionyx/api-gateway/internal/controller/config"
	"merionyx/api-gateway/internal/controller/domain/models"
	"merionyx/api-gateway/internal/controller/repository/memory"
	gwv1alpha1 "merionyx/api-gateway/pkg/apis/gateway/v1alpha1"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	cgscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	AnnEnvironmentID = "gateway.merionyx.io/environment-id"
	AnnServiceName     = "gateway.merionyx.io/service-name"
	AnnUpstream        = "gateway.merionyx.io/upstream"
	AnnPort            = "gateway.merionyx.io/port"
	LabelEnvironmentID = "gateway.merionyx.io/environment-id"
)

// Runner periodically lists Kubernetes resources and pushes a snapshot into in-memory repositories.
type Runner struct {
	client  client.Client
	cfg     *config.KubernetesDiscoveryConfig
	envRepo *memory.EnvironmentsRepository
	svcRepo *memory.ServiceRepository
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
func NewRunner(kd *config.KubernetesDiscoveryConfig, envRepo *memory.EnvironmentsRepository, svcRepo *memory.ServiceRepository) (*Runner, error) {
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

func (r *Runner) syncOnce(ctx context.Context) error {
	nsList, err := r.allowedNamespaces(ctx)
	if err != nil {
		return err
	}
	listOpts := r.listOpts()

	envs := make(map[string]*models.Environment)
	var globals []models.StaticServiceConfig

	for _, ns := range nsList {
		var elist gwv1alpha1.EnvironmentList
		if err := r.client.List(ctx, &elist, append(listOpts, client.InNamespace(ns))...); err != nil {
			return fmt.Errorf("list environments in %s: %w", ns, err)
		}
		for i := range elist.Items {
			e := &elist.Items[i]
			id := environmentLogicalName(e)
			envs[id] = newEnvModel(id)
		}
	}

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
		if envs[id] == nil {
			envs[id] = newEnvModel(id)
		}
		envs[id].Bundles.Static = append(envs[id].Bundles.Static, models.StaticContractBundleConfig{
			Name:       b.Spec.Name,
			Repository: b.Spec.Repository,
			Ref:        b.Spec.Ref,
			Path:       b.Spec.Path,
		})
	}

	var ulist gwv1alpha1.GatewayUpstreamList
	if err := r.client.List(ctx, &ulist, listOpts...); err != nil {
		return fmt.Errorf("list gatewayupstreams: %w", err)
	}
	for i := range ulist.Items {
		u := &ulist.Items[i]
		if u.Spec.EnvironmentID == "" {
			globals = append(globals, models.StaticServiceConfig{Name: u.Spec.Name, Upstream: u.Spec.Upstream})
			continue
		}
		id := u.Spec.EnvironmentID
		if envs[id] == nil {
			envs[id] = newEnvModel(id)
		}
		envs[id].Services.Static = append(envs[id].Services.Static, models.StaticServiceConfig{
			Name:     u.Spec.Name,
			Upstream: u.Spec.Upstream,
		})
	}

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
		if envs[envID] == nil {
			envs[envID] = newEnvModel(envID)
		}
		envs[envID].Services.Static = append(envs[envID].Services.Static, models.StaticServiceConfig{
			Name:     svcName,
			Upstream: up,
		})
	}

	r.svcRepo.SetKubernetesGlobalServices(globals)
	return r.envRepo.ApplyKubernetesEnvironments(ctx, envs)
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
