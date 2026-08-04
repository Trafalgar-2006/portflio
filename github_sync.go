package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/trafalgar-2006/ssh-portfolio/views"
)

// GitHub auto-sync
//
// A background worker polls the GitHub API and merges public repos into the
// project list, so pushing a new repo makes it appear on the portfolio without
// a redeploy. content.yaml stays authoritative: a YAML project is never
// overwritten, and `pinned: true` also stops a repo of the same name being
// added alongside it.
//
// Config:
//
//	GITHUB_USER   account to sync (default: trafalgar-2006)
//	GITHUB_TOKEN  optional PAT — raises the rate limit from 60/h to 5000/h
//	GITHUB_SYNC   set to "false" to disable the worker entirely
//	SYNC_INTERVAL poll interval (default 1h)

// syncState holds the most recent sync result for the admin/status views.
type syncState struct {
	mu        sync.RWMutex
	lastRun   time.Time
	lastErr   error
	added     int
	repoCount int
	rateLeft  string
}

var gitHubSync syncState

func (s *syncState) snapshot() (time.Time, error, int, int, string) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.lastRun, s.lastErr, s.added, s.repoCount, s.rateLeft
}

// ghRepo is the subset of the GitHub repo payload we consume.
type ghRepo struct {
	Name        string    `json:"name"`
	FullName    string    `json:"full_name"`
	Description string    `json:"description"`
	HTMLURL     string    `json:"html_url"`
	Language    string    `json:"language"`
	Topics      []string  `json:"topics"`
	Fork        bool      `json:"fork"`
	Archived    bool      `json:"archived"`
	Private     bool      `json:"private"`
	Stars       int       `json:"stargazers_count"`
	PushedAt    time.Time `json:"pushed_at"`
}

func envOr(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

// StartGitHubSync launches the polling worker unless disabled. It returns
// immediately; the first sync runs in the background.
func StartGitHubSync(ctx context.Context) {
	if strings.EqualFold(envOr("GITHUB_SYNC", "true"), "false") {
		log.Println("GitHub sync disabled (GITHUB_SYNC=false)")
		return
	}
	user := envOr("GITHUB_USER", "trafalgar-2006")

	interval := time.Hour
	if v := os.Getenv("SYNC_INTERVAL"); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d >= time.Minute {
			interval = d
		} else {
			log.Printf("SYNC_INTERVAL %q is not a duration >= 1m; using %s", v, interval)
		}
	}

	go func() {
		// Small delay so startup isn't blocked behind a network round trip.
		timer := time.NewTimer(5 * time.Second)
		defer timer.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-timer.C:
			}
			if err := syncGitHubOnce(ctx, user); err != nil {
				log.Printf("GitHub sync: %v", err)
			}
			timer.Reset(interval)
		}
	}()
}

// syncGitHubOnce fetches the user's repos and merges them into the live
// project list.
func syncGitHubOnce(ctx context.Context, user string) error {
	repos, rateLeft, err := fetchRepos(ctx, user)

	gitHubSync.mu.Lock()
	gitHubSync.lastRun = time.Now()
	gitHubSync.lastErr = err
	gitHubSync.rateLeft = rateLeft
	gitHubSync.mu.Unlock()

	if err != nil {
		return err
	}

	added := views.MergeGitHubRepos(toViewRepos(repos))

	gitHubSync.mu.Lock()
	gitHubSync.added = added
	gitHubSync.repoCount = len(repos)
	gitHubSync.mu.Unlock()

	log.Printf("GitHub sync: %d public repos, %d added to the project list (rate limit remaining: %s)",
		len(repos), added, rateLeft)
	return nil
}

func toViewRepos(repos []ghRepo) []views.GitHubRepo {
	out := make([]views.GitHubRepo, 0, len(repos))
	for _, r := range repos {
		out = append(out, views.GitHubRepo{
			Name:        r.Name,
			Description: r.Description,
			URL:         strings.TrimPrefix(r.HTMLURL, "https://"),
			Language:    r.Language,
			Topics:      r.Topics,
			Stars:       r.Stars,
			PushedAt:    r.PushedAt,
		})
	}
	return out
}

// fetchRepos pulls every public, non-fork, non-archived repo for a user,
// newest push first. Returns the remaining rate-limit budget for diagnostics.
func fetchRepos(ctx context.Context, user string) ([]ghRepo, string, error) {
	client := &http.Client{Timeout: 15 * time.Second}
	var all []ghRepo
	rateLeft := "unknown"

	// Paginate defensively — stop at 5 pages (500 repos).
	for page := 1; page <= 5; page++ {
		url := fmt.Sprintf("https://api.github.com/users/%s/repos?per_page=100&sort=pushed&page=%d", user, page)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return nil, rateLeft, err
		}
		req.Header.Set("Accept", "application/vnd.github+json")
		req.Header.Set("User-Agent", "ssh-portfolio")
		if tok := os.Getenv("GITHUB_TOKEN"); tok != "" {
			req.Header.Set("Authorization", "Bearer "+tok)
		}

		resp, err := client.Do(req)
		if err != nil {
			return nil, rateLeft, err
		}
		if v := resp.Header.Get("X-RateLimit-Remaining"); v != "" {
			rateLeft = v
		}
		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			if resp.StatusCode == http.StatusForbidden {
				return nil, rateLeft, fmt.Errorf("rate limited (set GITHUB_TOKEN to raise the limit)")
			}
			return nil, rateLeft, fmt.Errorf("GitHub API returned %s", resp.Status)
		}

		var page1 []ghRepo
		err = json.NewDecoder(resp.Body).Decode(&page1)
		resp.Body.Close()
		if err != nil {
			return nil, rateLeft, fmt.Errorf("decoding repo list: %w", err)
		}
		if len(page1) == 0 {
			break
		}
		for _, r := range page1 {
			if r.Fork || r.Archived || r.Private {
				continue // forks and archives aren't portfolio material
			}
			all = append(all, r)
		}
		if len(page1) < 100 {
			break
		}
	}

	sort.Slice(all, func(i, j int) bool { return all[i].PushedAt.After(all[j].PushedAt) })
	return all, rateLeft, nil
}
