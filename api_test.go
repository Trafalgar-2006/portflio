package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/trafalgar-2006/ssh-portfolio/views"
)

// The web page renders straight from this payload, so a shape change here
// silently breaks the site. Pin the contract.
func TestContentAPIShape(t *testing.T) {
	saved := views.Projects()
	t.Cleanup(func() { views.SetProjects(saved) })
	views.SetProjects([]views.Project{
		{Title: "Curated Thing", Description: "d", Tags: []string{"Go"}, Status: "Live", GitHubURL: "github.com/u/x"},
		{Title: "auto-repo", Description: "d", Status: "Research", FromGitHub: true},
		{Title: "No Tags", Description: "d", Status: "WIP"},
	})

	rec := httptest.NewRecorder()
	handleAPIContent(rec, httptest.NewRequest(http.MethodGet, "/api/content", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json; charset=utf-8" {
		t.Errorf("Content-Type = %q", ct)
	}

	var got apiContent
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("response is not valid JSON: %v", err)
	}
	if len(got.Projects) != 3 {
		t.Fatalf("got %d projects, want 3", len(got.Projects))
	}

	// The page's CSS keys off these exact status slugs.
	wantSlugs := map[string]string{"Curated Thing": "live", "auto-repo": "research", "No Tags": "wip"}
	for _, p := range got.Projects {
		if want := wantSlugs[p.Title]; p.Status != want {
			t.Errorf("%s: status %q, want %q", p.Title, p.Status, want)
		}
		// A nil slice would serialise as null and break .map() in the browser.
		if p.Tags == nil {
			t.Errorf("%s: tags serialised as null, must be []", p.Title)
		}
	}
	if !got.Projects[1].FromGitHub {
		t.Error("auto-synced project not flagged")
	}
	if got.Projects[0].FromGitHub {
		t.Error("curated project wrongly flagged as auto-synced")
	}
	if got.Name == "" || got.Generated == "" {
		t.Error("profile name / timestamp missing")
	}
	if _, err := time.Parse(time.RFC3339, got.Generated); err != nil {
		t.Errorf("generated timestamp not RFC3339: %v", err)
	}
}

// An empty project list must still produce valid JSON with an array, not null.
func TestContentAPIEmpty(t *testing.T) {
	saved := views.Projects()
	t.Cleanup(func() { views.SetProjects(saved) })
	views.SetProjects(nil)

	rec := httptest.NewRecorder()
	handleAPIContent(rec, httptest.NewRequest(http.MethodGet, "/api/content", nil))

	var got map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if got["projects"] == nil {
		t.Error("projects serialised as null; the browser would throw on .map()")
	}
}
