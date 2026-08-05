package main

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/trafalgar-2006/ssh-portfolio/views"
)

// Content API
//
// The web portfolio used to carry its own hardcoded copy of the project list
// with a "keep in sync" comment on top, which is exactly the kind of thing
// that stops being in sync. It now fetches this endpoint instead, so the page
// reflects content.yaml — and anything the GitHub sync worker has added —
// without a rebuild.
//
// The hardcoded array stays in index.html as a fallback, so opening the file
// directly (or a failed fetch) still renders a complete page.

// apiProject is one project as the web page consumes it.
type apiProject struct {
	Title       string   `json:"title"`
	Description string   `json:"desc"`
	Tags        []string `json:"tags"`
	Status      string   `json:"status"` // live | wip | research
	Label       string   `json:"label"`  // Live | WIP | Research
	Highlight   string   `json:"hl"`
	GitHub      string   `json:"github"`
	FromGitHub  bool     `json:"auto"` // true when added by the sync worker
}

// apiContact mirrors a contact row.
type apiContact struct {
	Icon  string `json:"icon"`
	Label string `json:"label"`
	Value string `json:"value"`
}

// apiContent is the whole payload.
type apiContent struct {
	Name      string       `json:"name"`
	Tagline   string       `json:"tagline"`
	Location  string       `json:"location"`
	ResumeURL string       `json:"resumeUrl"`
	Projects  []apiProject `json:"projects"`
	Contacts  []apiContact `json:"contacts"`
	Commit    string       `json:"commit"`
	Generated string       `json:"generated"`
}

// statusSlug maps the TUI's status wording to the CSS class the page uses.
func statusSlug(s string) (slug, label string) {
	switch s {
	case "Live":
		return "live", "Live"
	case "WIP":
		return "wip", "WIP"
	case "Research":
		return "research", "Research"
	default:
		return "wip", s
	}
}

// buildAPIContent snapshots the live content for the web page.
func buildAPIContent() apiContent {
	projects := views.Projects() // locked snapshot; includes GitHub-synced entries
	out := make([]apiProject, 0, len(projects))
	for _, p := range projects {
		slug, label := statusSlug(p.Status)
		tags := p.Tags
		if tags == nil {
			tags = []string{}
		}
		out = append(out, apiProject{
			Title:       p.Title,
			Description: p.Description,
			Tags:        tags,
			Status:      slug,
			Label:       label,
			Highlight:   p.Highlight,
			GitHub:      p.GitHubURL,
			FromGitHub:  p.FromGitHub,
		})
	}

	contacts := make([]apiContact, 0, len(views.AllContacts))
	for _, c := range views.AllContacts {
		contacts = append(contacts, apiContact{Icon: c.Icon, Label: c.Label, Value: c.Value})
	}

	return apiContent{
		Name:      views.TheProfile.Name,
		Tagline:   views.TheProfile.Tagline,
		Location:  views.TheProfile.Location,
		ResumeURL: views.TheProfile.ResumeURL,
		Projects:  out,
		Contacts:  contacts,
		Commit:    BuildCommit,
		Generated: time.Now().UTC().Format(time.RFC3339),
	}
}

// handleAPIContent serves the live content as JSON.
func handleAPIContent(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	// Short cache: long enough to spare the server on a refresh burst, short
	// enough that a new repo shows up on the site within a minute.
	w.Header().Set("Cache-Control", "public, max-age=60")
	w.Header().Set("Access-Control-Allow-Origin", "*") // read-only public data

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if err := enc.Encode(buildAPIContent()); err != nil {
		http.Error(w, "could not encode content", http.StatusInternalServerError)
	}
}
