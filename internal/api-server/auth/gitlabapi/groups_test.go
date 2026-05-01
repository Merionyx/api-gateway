package gitlabapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestListMembershipGroupFullPaths(t *testing.T) {
	t.Parallel()
	pages := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v4/groups" {
			http.NotFound(w, r)
			return
		}
		pages++
		page := r.URL.Query().Get("page")
		if r.URL.Query().Get("membership") != "true" {
			http.Error(w, "want membership=true", http.StatusBadRequest)
			return
		}
		if page == "1" {
			batch := []groupWire{{FullPath: "acme/platform"}}
			for i := 0; i < 99; i++ {
				batch = append(batch, groupWire{FullPath: "x"})
			}
			_ = json.NewEncoder(w).Encode(batch)
			return
		}
		_ = json.NewEncoder(w).Encode([]groupWire{{FullPath: "acme/other"}})
	}))
	t.Cleanup(srv.Close)

	base := srv.URL + "/api/v4"
	got, err := ListMembershipGroupFullPaths(t.Context(), srv.Client(), "glpat-test", base)
	if err != nil {
		t.Fatal(err)
	}
	if pages != 2 {
		t.Fatalf("pages=%d", pages)
	}
	if len(got) < 2 {
		t.Fatalf("paths=%d", len(got))
	}
}
