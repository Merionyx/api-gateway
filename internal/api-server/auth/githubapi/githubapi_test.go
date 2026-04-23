package githubapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestListUserOrgLogins_paginated(t *testing.T) {
	t.Parallel()
	pages := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/user/orgs" {
			http.NotFound(w, r)
			return
		}
		pages++
		page := r.URL.Query().Get("page")
		if page == "1" {
			batch := []orgWire{{Login: "a"}, {Login: "b"}}
			for i := 0; i < 98; i++ {
				batch = append(batch, orgWire{Login: "x"})
			}
			_ = json.NewEncoder(w).Encode(batch)
			return
		}
		if page == "2" {
			_ = json.NewEncoder(w).Encode([]orgWire{{Login: "c"}})
			return
		}
		_ = json.NewEncoder(w).Encode([]orgWire{})
	}))
	t.Cleanup(srv.Close)

	got, err := ListUserOrgLogins(t.Context(), srv.Client(), "tok", srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	if pages != 2 {
		t.Fatalf("pages=%d", pages)
	}
	if len(got) < 3 {
		t.Fatalf("orgs=%d", len(got))
	}
}

func TestListUserTeams(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/user/teams" {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode([]teamWire{{
			Slug: "eng",
			Organization: struct {
				Login string `json:"login"`
			}{Login: "acme"},
		}})
	}))
	t.Cleanup(srv.Close)

	got, err := ListUserTeams(t.Context(), srv.Client(), "tok", srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].OrgLogin != "acme" || got[0].Slug != "eng" {
		t.Fatalf("%+v", got)
	}
}
