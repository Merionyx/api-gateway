package cache

import (
	"fmt"
	"testing"

	"github.com/merionyx/api-gateway/internal/controller/repository/etcd"
	"github.com/merionyx/api-gateway/internal/shared/bundlekey"
)

func TestClassifyControllerEtcdWatchKey(t *testing.T) {
	// SchemaPrefix is already .../schemas/ — next segment is repository name.
	schemaKey := fmt.Sprintf("%smyrepo/main/./contracts/c1/snapshot", etcd.SchemaPrefix)
	bk := bundlekey.Build("myrepo", "main", "")

	tests := []struct {
		name string
		key  string
		want ControllerEtcdKeyEffect
	}{
		{
			name: "election_ignored",
			key:  etcd.ControllerWatchPrefix + "election/leader",
			want: ControllerEtcdKeyEffect{Ignore: true},
		},
		{
			name: "schema_contract",
			key:  schemaKey,
			want: ControllerEtcdKeyEffect{SchemaBundleKey: bk},
		},
		{
			name: "environment_config",
			key:  etcd.EnvironmentPrefix + "staging/config",
			want: ControllerEtcdKeyEffect{Environment: "staging"},
		},
		{
			name: "materialized_effective_ignored",
			key:  etcd.EffectiveMaterializedPrefix + "dev/v1",
			want: ControllerEtcdKeyEffect{Ignore: true},
		},
		{
			name: "unknown_under_controller",
			key:  etcd.ControllerWatchPrefix + "other/sub/key",
			want: ControllerEtcdKeyEffect{UnknownUnderPrefix: true},
		},
		{
			name: "outside_controller",
			key:  "/api-gateway/api-server/foo",
			want: ControllerEtcdKeyEffect{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ClassifyControllerEtcdWatchKey(tt.key)
			if got != tt.want {
				t.Fatalf("got %+v want %+v", got, tt.want)
			}
		})
	}
}
