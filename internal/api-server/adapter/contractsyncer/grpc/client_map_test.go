package grpc

import (
	"strconv"
	"testing"

	commonv1 "github.com/merionyx/api-gateway/pkg/grpc/common/v1"
)

func TestMapCommonSnapshotsToDomain_emptyAndNil(t *testing.T) {
	t.Parallel()
	if g := mapCommonSnapshotsToDomain(nil); len(g) != 0 {
		t.Fatalf("nil in: %v", g)
	}
	if g := mapCommonSnapshotsToDomain([]*commonv1.ContractSnapshot{nil, {Name: "only"}}); len(g) != 1 || g[0].Name != "only" {
		t.Fatalf("got %#v", g)
	}
}

func TestMapCommonSnapshotsToDomain_full(t *testing.T) {
	t.Parallel()
	in := []*commonv1.ContractSnapshot{{
		Name:                  "api",
		Prefix:                "/p",
		AllowUndefinedMethods: true,
		Upstream:              &commonv1.ContractUpstream{Name: "up1"},
		Access: &commonv1.Access{
			Secure: true,
			Apps: []*commonv1.App{
				{AppId: "app1", Environments: []string{"e1", "e2"}},
			},
		},
	}}
	out := mapCommonSnapshotsToDomain(in)
	if len(out) != 1 {
		t.Fatalf("len %d", len(out))
	}
	s := out[0]
	if s.Name != "api" || s.Prefix != "/p" || !s.AllowUndefinedMethods {
		t.Fatalf("meta: %#v", s)
	}
	if s.Upstream.Name != "up1" {
		t.Fatalf("upstream: %#v", s.Upstream)
	}
	if !s.Access.Secure || len(s.Access.Apps) != 1 || s.Access.Apps[0].AppID != "app1" {
		t.Fatalf("access: %#v", s.Access)
	}
}

func TestMapCommonSnapshotsToDomain_noUpstreamName(t *testing.T) {
	t.Parallel()
	out := mapCommonSnapshotsToDomain([]*commonv1.ContractSnapshot{{
		Name:   "n",
		Access: &commonv1.Access{Secure: false},
	}})
	if len(out) != 1 || out[0].Upstream.Name != "" {
		t.Fatalf("%#v", out[0].Upstream)
	}
}

func TestMapCommonSnapshotsToDomain_manySnapshots(t *testing.T) {
	t.Parallel()
	var in []*commonv1.ContractSnapshot
	for i := range 8 {
		in = append(in, &commonv1.ContractSnapshot{
			Name:   "s" + strconv.Itoa(i),
			Prefix: "/",
			Access: &commonv1.Access{Apps: []*commonv1.App{
				{AppId: "a", Environments: []string{"e"}},
			}},
		})
	}
	out := mapCommonSnapshotsToDomain(in)
	if len(out) != 8 {
		t.Fatalf("len %d", len(out))
	}
}
