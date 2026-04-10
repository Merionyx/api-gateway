package usecase

import (
	"testing"

	"github.com/merionyx/api-gateway/internal/api-server/domain/models"
	"github.com/merionyx/api-gateway/internal/shared/bundlekey"
)

func TestCollectUniqueBundles_dedupesByBundleKey(t *testing.T) {
	ctr := []models.ControllerInfo{
		{
			Environments: []models.EnvironmentInfo{
				{
					Bundles: []models.BundleInfo{
						{Name: "a", Repository: "r", Ref: "main", Path: "p"},
					},
				},
				{
					Bundles: []models.BundleInfo{
						{Name: "dup", Repository: "r", Ref: "main", Path: "p"},
					},
				},
			},
		},
	}
	got := collectUniqueBundles(ctr)
	if len(got) != 1 {
		t.Fatalf("want 1 unique bundle, got %d", len(got))
	}
	wantKey := bundlekey.Build("r", "main", "p")
	gotKey := bundlekey.Build(got[0].Repository, got[0].Ref, got[0].Path)
	if gotKey != wantKey {
		t.Fatalf("key %q want %q", gotKey, wantKey)
	}
}
