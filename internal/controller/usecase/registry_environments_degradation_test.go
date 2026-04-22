package usecase

import (
	"errors"
	"reflect"
	"testing"
)

func TestCountWarningKinds(t *testing.T) {
	t.Parallel()
	w := []RegistryEnvironmentsBuildWarning{
		{Kind: RegistryBuildWarningInMemoryList},
		{Kind: RegistryBuildWarningInMemoryList},
		{Kind: RegistryBuildWarningEtcdList},
		{Kind: ""},
	}
	want := map[string]int{RegistryBuildWarningInMemoryList: 2, RegistryBuildWarningEtcdList: 1}
	if got := countWarningKinds(w); !reflect.DeepEqual(got, want) {
		t.Fatalf("countWarningKinds: got %v, want %v", got, want)
	}
}

func TestRegistryEnvironmentsBuildReportAddWarning(t *testing.T) {
	t.Parallel()
	var r RegistryEnvironmentsBuildReport
	r.addWarning(RegistryBuildWarningEnvMerge, "dev", nil)
	if len(r.Warnings) != 0 {
		t.Fatalf("expected nil err to skip, got %d", len(r.Warnings))
	}
	r.addWarning(RegistryBuildWarningEnvMerge, "stg", errors.New("boom"))
	if len(r.Warnings) != 1 || r.Warnings[0].Kind != RegistryBuildWarningEnvMerge {
		t.Fatalf("unexpected %#v", r.Warnings)
	}
}
