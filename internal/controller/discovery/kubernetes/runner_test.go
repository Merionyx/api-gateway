package kubernetes

import (
	"context"
	"testing"

	"github.com/merionyx/api-gateway/internal/controller/config"
	gwv1alpha1 "github.com/merionyx/api-gateway/pkg/apis/crd/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	cgscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestLabelsMatch(t *testing.T) {
	t.Parallel()
	if !labelsMatch(map[string]string{"a": "1", "b": "2"}, map[string]string{"a": "1", "b": "2", "c": "3"}) {
		t.Fatal("expected match")
	}
	if labelsMatch(map[string]string{"a": "1"}, map[string]string{"a": "2"}) {
		t.Fatal("expected no match")
	}
	if !labelsMatch(nil, map[string]string{"a": "1"}) {
		t.Fatal("empty sel matches any")
	}
}

func TestEnvironmentLogicalName(t *testing.T) {
	t.Parallel()
	e := &gwv1alpha1.Environment{
		ObjectMeta: metav1.ObjectMeta{Namespace: "ns", Name: "x"},
		Spec:       gwv1alpha1.EnvironmentSpec{LogicalName: "custom"},
	}
	if g, w := environmentLogicalName(e), "custom"; g != w {
		t.Fatalf("got %q want %q", g, w)
	}
	e2 := &gwv1alpha1.Environment{
		ObjectMeta: metav1.ObjectMeta{Namespace: "a", Name: "b"},
	}
	if g, w := environmentLogicalName(e2), "a-b"; g != w {
		t.Fatalf("got %q want %q", g, w)
	}
}

func TestNewEnvModel(t *testing.T) {
	t.Parallel()
	e := newEnvModel("e1")
	if e.Name != "e1" || e.Type != "kubernetes" {
		t.Fatalf("bad env: %#v", e)
	}
	if e.Bundles == nil || e.Services == nil {
		t.Fatal("expected bundles and services inited")
	}
}

func TestUpstreamFromService(t *testing.T) {
	t.Parallel()
	s := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: "s", Namespace: "ns", Annotations: map[string]string{AnnPort: "9090"}},
		Spec:       corev1.ServiceSpec{Ports: []corev1.ServicePort{{Port: 443}}},
	}
	if g := upstreamFromService(s); g != "http://s.ns.svc.cluster.local:9090" {
		t.Fatalf("got %q", g)
	}
	s2 := &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "x", Namespace: "y"}, Spec: corev1.ServiceSpec{Ports: []corev1.ServicePort{{Port: 3000}}}}
	if g := upstreamFromService(s2); g != "http://x.y.svc.cluster.local:3000" {
		t.Fatalf("got %q", g)
	}
}

func TestRunner_listOpts(t *testing.T) {
	t.Parallel()
	r := &Runner{cfg: &config.KubernetesDiscoveryConfig{}}
	if r.listOpts() != nil {
		t.Fatalf("want nil, got %v", r.listOpts())
	}
	r2 := &Runner{cfg: &config.KubernetesDiscoveryConfig{ResourceLabelSelector: map[string]string{"k": "v"}}}
	lo := r2.listOpts()
	if len(lo) != 1 {
		t.Fatalf("len %d", len(lo))
	}
}

func TestBundleEnvID_branches(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	r := &Runner{} // no client: only early branches
	b1 := &gwv1alpha1.ContractBundle{Spec: gwv1alpha1.ContractBundleSpec{EnvironmentID: "eid"}}
	if id, err := r.bundleEnvID(ctx, b1); err != nil || id != "eid" {
		t.Fatalf("got %q %v", id, err)
	}
	b2 := &gwv1alpha1.ContractBundle{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{LabelEnvironmentID: "from-label"}}}
	if id, err := r.bundleEnvID(ctx, b2); err != nil || id != "from-label" {
		t.Fatalf("got %q %v", id, err)
	}
	if _, err := r.bundleEnvID(ctx, &gwv1alpha1.ContractBundle{Spec: gwv1alpha1.ContractBundleSpec{}}); err == nil {
		t.Fatal("expected error without ref/label")
	}
}

func TestBundleEnvID_fromEnvironmentRef(t *testing.T) {
	t.Parallel()
	sch := runtime.NewScheme()
	_ = cgscheme.AddToScheme(sch)
	_ = gwv1alpha1.AddToScheme(sch)
	env := &gwv1alpha1.Environment{
		ObjectMeta: metav1.ObjectMeta{Namespace: "n", Name: "e"},
		Spec:       gwv1alpha1.EnvironmentSpec{LogicalName: "from-ref"},
	}
	cl := fake.NewClientBuilder().WithScheme(sch).WithObjects(env).Build()
	r := &Runner{client: cl}
	b := &gwv1alpha1.ContractBundle{
		ObjectMeta: metav1.ObjectMeta{Namespace: "n", Name: "b"},
		Spec: gwv1alpha1.ContractBundleSpec{
			EnvironmentRef: &corev1.ObjectReference{Namespace: "n", Name: "e"},
		},
	}
	id, err := r.bundleEnvID(context.Background(), b)
	if err != nil || id != "from-ref" {
		t.Fatalf("got %q %v", id, err)
	}
}

func TestBundleEnvID_environmentRefGetError(t *testing.T) {
	t.Parallel()
	sch := runtime.NewScheme()
	_ = cgscheme.AddToScheme(sch)
	_ = gwv1alpha1.AddToScheme(sch)
	r := &Runner{client: fake.NewClientBuilder().WithScheme(sch).Build()}
	b := &gwv1alpha1.ContractBundle{
		ObjectMeta: metav1.ObjectMeta{Namespace: "n", Name: "b"},
		Spec: gwv1alpha1.ContractBundleSpec{
			EnvironmentRef: &corev1.ObjectReference{Namespace: "n", Name: "missing"},
		},
	}
	_, err := r.bundleEnvID(context.Background(), b)
	if err == nil {
		t.Fatal("expected get error")
	}
}
