package views

import (
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"
)

func withProjects(t *testing.T, ps []Project) {
	t.Helper()
	saved := AllProjects
	AllProjects = ps
	t.Cleanup(func() { AllProjects = saved })
}

// A repo already curated in content.yaml must never be added a second time.
func TestMergeSkipsCuratedProjects(t *testing.T) {
	withProjects(t, []Project{
		{Title: "EmbedGen — Domain-Specific Code LLM", GitHubURL: "github.com/trafalgar-2006/EmbedGen"},
		{Title: "BioAcoustic Frog Classifier", GitHubURL: "github.com/trafalgar-2006/BioAcoustic-Frog-Classifier"},
		{Title: "Webcraft Studios — Digital Agency"}, // no repo at all
	})

	added := MergeGitHubRepos([]GitHubRepo{
		{Name: "EmbedGen", URL: "github.com/trafalgar-2006/EmbedGen", PushedAt: time.Now()},
		{Name: "BioAcoustic-Frog-Classifier", URL: "github.com/trafalgar-2006/BioAcoustic-Frog-Classifier", PushedAt: time.Now()},
		{Name: "brand-new-repo", URL: "github.com/trafalgar-2006/brand-new-repo", PushedAt: time.Now()},
	})

	if added != 1 {
		t.Errorf("added %d projects, want 1 (only the new repo)", added)
	}
	if len(AllProjects) != 4 {
		t.Fatalf("project list has %d entries, want 4", len(AllProjects))
	}
	// Curated entries must keep their curated titles.
	if AllProjects[0].Title != "EmbedGen — Domain-Specific Code LLM" {
		t.Errorf("curated title was overwritten: %q", AllProjects[0].Title)
	}
	if AllProjects[3].Title != "brand-new-repo" {
		t.Errorf("new repo not appended, got %q", AllProjects[3].Title)
	}
}

// Re-syncing must refresh the auto set, not duplicate it.
func TestMergeIsIdempotent(t *testing.T) {
	withProjects(t, []Project{{Title: "Curated Thing"}})

	repos := []GitHubRepo{
		{Name: "repo-a", URL: "github.com/u/repo-a", PushedAt: time.Now()},
		{Name: "repo-b", URL: "github.com/u/repo-b", PushedAt: time.Now()},
	}
	MergeGitHubRepos(repos)
	first := len(AllProjects)

	for i := 0; i < 5; i++ {
		MergeGitHubRepos(repos)
	}
	if len(AllProjects) != first {
		t.Errorf("repeated syncs grew the list: %d -> %d", first, len(AllProjects))
	}
	// The curated project must survive every sync.
	if AllProjects[0].Title != "Curated Thing" {
		t.Errorf("curated project lost after re-sync: %q", AllProjects[0].Title)
	}
}

// A repo removed from GitHub must disappear on the next sync.
func TestMergeDropsVanishedRepos(t *testing.T) {
	withProjects(t, []Project{{Title: "Curated Thing"}})

	MergeGitHubRepos([]GitHubRepo{
		{Name: "keep-me", URL: "github.com/u/keep-me", PushedAt: time.Now()},
		{Name: "delete-me", URL: "github.com/u/delete-me", PushedAt: time.Now()},
	})
	MergeGitHubRepos([]GitHubRepo{
		{Name: "keep-me", URL: "github.com/u/keep-me", PushedAt: time.Now()},
	})

	for _, p := range AllProjects {
		if p.Title == "delete-me" {
			t.Error("a repo removed from GitHub survived the next sync")
		}
	}
	if len(AllProjects) != 2 {
		t.Errorf("expected curated + 1 synced, got %d", len(AllProjects))
	}
}

// Duplicates inside a single batch must collapse.
func TestMergeDedupesWithinBatch(t *testing.T) {
	withProjects(t, nil)
	added := MergeGitHubRepos([]GitHubRepo{
		{Name: "same-repo", URL: "github.com/u/same-repo", PushedAt: time.Now()},
		{Name: "same-repo", URL: "github.com/u/same-repo", PushedAt: time.Now()},
	})
	if added != 1 {
		t.Errorf("added %d, want 1 — duplicates in one batch should collapse", added)
	}
}

// Status must follow push recency, and a missing description must not render
// as an empty panel.
func TestProjectFromRepoFields(t *testing.T) {
	now := time.Now()
	cases := []struct {
		pushed time.Time
		want   string
	}{
		{now.Add(-2 * 24 * time.Hour), "Live"},
		{now.Add(-90 * 24 * time.Hour), "WIP"},
		{now.Add(-400 * 24 * time.Hour), "Research"},
	}
	for _, tc := range cases {
		p := projectFromRepo(GitHubRepo{Name: "x", PushedAt: tc.pushed})
		if p.Status != tc.want {
			t.Errorf("pushed %v: status %q, want %q", tc.pushed, p.Status, tc.want)
		}
		if strings.TrimSpace(p.Description) == "" {
			t.Error("empty description would render as a blank detail panel")
		}
		if !p.FromGitHub {
			t.Error("synced project not marked FromGitHub")
		}
	}

	// Language leads the tag list; topics follow; the list stays bounded.
	p := projectFromRepo(GitHubRepo{
		Name: "x", Language: "Go",
		Topics:   []string{"go", "cli", "tui", "ssh", "terminal", "portfolio", "extra", "more"},
		PushedAt: now,
	})
	if len(p.Tags) == 0 || p.Tags[0] != "Go" {
		t.Errorf("expected Go to lead the tags, got %v", p.Tags)
	}
	if len(p.Tags) > 6 {
		t.Errorf("tag list not capped: %d tags", len(p.Tags))
	}
	// Stars become a highlight.
	if got := projectFromRepo(GitHubRepo{Name: "x", Stars: 12, PushedAt: now}); !strings.Contains(got.Highlight, "12") {
		t.Errorf("star count missing from highlight: %q", got.Highlight)
	}
}

// Name matching must survive punctuation differences between a curated title
// and the repo slug.
func TestRepoAlreadyListedNormalises(t *testing.T) {
	curated := []Project{
		{Title: "BioAcoustic Frog Classifier", GitHubURL: "github.com/u/BioAcoustic-Frog-Classifier"},
	}
	for _, name := range []string{"BioAcoustic-Frog-Classifier", "bioacoustic_frog_classifier", "BioAcousticFrogClassifier"} {
		if !repoAlreadyListed(GitHubRepo{Name: name, URL: "github.com/u/" + name}, curated) {
			t.Errorf("%q was not matched against the curated entry", name)
		}
	}
	if repoAlreadyListed(GitHubRepo{Name: "totally-different", URL: "github.com/u/totally-different"}, curated) {
		t.Error("an unrelated repo was wrongly treated as a duplicate")
	}
}

// The worker writes AllProjects from its own goroutine while sessions render.
// Run under -race to prove the lock actually covers it.
func TestMergeIsRaceFree(t *testing.T) {
	withProjects(t, []Project{{Title: "Curated"}})

	var wg sync.WaitGroup
	stop := make(chan struct{})

	// Writer: repeated syncs.
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; ; i++ {
			select {
			case <-stop:
				return
			default:
			}
			MergeGitHubRepos([]GitHubRepo{
				{Name: "r1", URL: "github.com/u/r1", PushedAt: time.Now()},
				{Name: "r2", URL: "github.com/u/r2", PushedAt: time.Now()},
			})
		}
	}()

	// Readers: snapshot and render, like live SSH sessions.
	r := lipgloss.DefaultRenderer()
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
				}
				ps := Projects()
				for _, p := range ps {
					_ = p.Title
				}
				RenderProjects(r, 120, 40, 0, 0, 99, 99, true, 0, 0, nil, 5, -1, 0, -1, 0, ThemeDracula)
			}
		}()
	}

	time.Sleep(150 * time.Millisecond)
	close(stop)
	wg.Wait()
}
