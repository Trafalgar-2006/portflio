package views

import (
	"strings"
	"sync"
	"time"
)

// GitHubRepo is a repo as the sync worker sees it.
type GitHubRepo struct {
	Name        string
	Description string
	URL         string
	Language    string
	Topics      []string
	Stars       int
	PushedAt    time.Time
}

// projectsMu guards AllProjects. The GitHub worker writes from its own
// goroutine while SSH sessions read it to render, so the slice can't be
// touched unsynchronised.
var projectsMu sync.RWMutex

// Projects returns a snapshot of the project list safe to read while the
// sync worker is running. Every reader outside this file must go through
// Projects/ProjectCount/ProjectAt — reading AllProjects directly races with
// the background GitHub sync.
func Projects() []Project {
	projectsMu.RLock()
	defer projectsMu.RUnlock()
	out := make([]Project, len(AllProjects))
	copy(out, AllProjects)
	return out
}

// ProjectCount is the number of projects currently listed.
func ProjectCount() int {
	projectsMu.RLock()
	defer projectsMu.RUnlock()
	return len(AllProjects)
}

// ProjectAt returns the project at index i, reporting false when i is out of
// range — the list can shrink between a keypress and the next render.
func ProjectAt(i int) (Project, bool) {
	projectsMu.RLock()
	defer projectsMu.RUnlock()
	if i < 0 || i >= len(AllProjects) {
		return Project{}, false
	}
	return AllProjects[i], true
}

// SetProjects replaces the project list under the lock.
func SetProjects(ps []Project) {
	projectsMu.Lock()
	defer projectsMu.Unlock()
	AllProjects = ps
}

// normaliseKey reduces a title or repo name to a comparison key, so
// "BioAcoustic-Frog-Classifier" and "BioAcoustic Frog Classifier" match.
func normaliseKey(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		}
	}
	return b.String()
}

// repoAlreadyListed reports whether a repo is already represented in the
// project list, by URL or by name similarity.
func repoAlreadyListed(repo GitHubRepo, projects []Project) bool {
	repoKey := normaliseKey(repo.Name)
	urlKey := strings.ToLower(strings.TrimSuffix(repo.URL, "/"))

	for _, p := range projects {
		if p.GitHubURL != "" {
			if strings.EqualFold(strings.TrimSuffix(p.GitHubURL, "/"), urlKey) {
				return true
			}
			// Compare the trailing path segment (the repo name).
			if i := strings.LastIndex(p.GitHubURL, "/"); i >= 0 {
				if normaliseKey(p.GitHubURL[i+1:]) == repoKey {
					return true
				}
			}
		}
		// A curated title often embeds the repo name ("EmbedGen — …").
		titleKey := normaliseKey(p.Title)
		if titleKey == repoKey {
			return true
		}
		if len(repoKey) >= 5 && strings.Contains(titleKey, repoKey) {
			return true
		}
	}
	return false
}

// projectFromRepo converts a GitHub repo into a portfolio project.
func projectFromRepo(repo GitHubRepo) Project {
	desc := strings.TrimSpace(repo.Description)
	if desc == "" {
		desc = "A public repository. No description set on GitHub yet — " +
			"add one and it'll appear here on the next sync."
	}

	// Tags: primary language first, then GitHub topics, capped so a
	// heavily-tagged repo can't blow out the detail panel.
	var tags []string
	if repo.Language != "" {
		tags = append(tags, repo.Language)
	}
	for _, t := range repo.Topics {
		if len(tags) >= 6 {
			break
		}
		if !strings.EqualFold(t, repo.Language) {
			tags = append(tags, t)
		}
	}

	// Status from recency: actively pushed repos read as live.
	status := "Research"
	switch age := time.Since(repo.PushedAt); {
	case age < 30*24*time.Hour:
		status = "Live"
	case age < 180*24*time.Hour:
		status = "WIP"
	}

	highlight := ""
	if repo.Stars >= 5 {
		highlight = "★ " + itoa(repo.Stars)
	}

	return Project{
		Title:       repo.Name,
		Description: desc,
		Tags:        tags,
		Status:      status,
		GitHubURL:   repo.URL,
		Highlight:   highlight,
		FromGitHub:  true,
	}
}

// MergeGitHubRepos appends repos that aren't already represented in the
// project list, and refreshes the ones previously added by a sync. Projects
// that came from content.yaml are never modified — the YAML stays
// authoritative for anything you've curated.
//
// Returns how many new projects were added.
func MergeGitHubRepos(repos []GitHubRepo) int {
	projectsMu.Lock()
	defer projectsMu.Unlock()

	// Split curated (YAML/built-in) from previously auto-synced entries, so a
	// re-sync replaces the auto set rather than duplicating it.
	curated := make([]Project, 0, len(AllProjects))
	for _, p := range AllProjects {
		if !p.FromGitHub {
			curated = append(curated, p)
		}
	}

	var synced []Project
	for _, repo := range repos {
		if repo.Name == "" {
			continue
		}
		if repoAlreadyListed(repo, curated) {
			continue // curated entry wins
		}
		// Don't add a repo twice within one batch.
		if repoAlreadyListed(repo, synced) {
			continue
		}
		synced = append(synced, projectFromRepo(repo))
	}

	AllProjects = append(curated, synced...)
	return len(synced)
}
