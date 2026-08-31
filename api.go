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

// apiRole is one entry in the experience timeline.
type apiRole struct {
	Title    string   `json:"title"`
	Org      string   `json:"org"`
	Period   string   `json:"period"`
	Location string   `json:"location"`
	Bullets  []string `json:"bullets"`
}

// apiEducation is a degree or programme.
type apiEducation struct {
	School  string   `json:"school"`
	Degree  string   `json:"degree"`
	Period  string   `json:"period"`
	Note    string   `json:"note"`
	Bullets []string `json:"bullets"`
}

// apiSkill is a labelled proficiency bar.
type apiSkill struct {
	Name     string `json:"name"`
	Pct      int    `json:"pct"`
	Category string `json:"category"`
}

// apiSkillGroup is a resume-style category listing.
type apiSkillGroup struct {
	Category string `json:"category"`
	Items    string `json:"items"`
}

// apiNow is the /now page body.
type apiNow struct {
	Building []string `json:"building"`
	Learning []string `json:"learning"`
	Reading  []string `json:"reading"`
	Status   []string `json:"status"`
}

// apiContent is the whole payload.
//
// This carries everything content.yaml holds, not just the projects. The web
// page used to keep its own hand-typed copy of the bio, the jobs, the degree
// and the skills, so editing content.yaml updated the TUI and quietly left the
// site a version behind. Now both surfaces read the same thing.
type apiContent struct {
	Name        string          `json:"name"`
	Tagline     string          `json:"tagline"`
	Location    string          `json:"location"`
	Headline    string          `json:"headline"`
	Drive       []string        `json:"drive"`
	ResumeURL   string          `json:"resumeUrl"`
	Projects    []apiProject    `json:"projects"`
	Contacts    []apiContact    `json:"contacts"`
	Experience  []apiRole       `json:"experience"`
	Education   []apiEducation  `json:"education"`
	Skills      []apiSkill      `json:"skills"`
	SkillGroups []apiSkillGroup `json:"skillGroups"`
	Now         apiNow          `json:"now"`
	Commit      string          `json:"commit"`
	Generated   string          `json:"generated"`
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

	experience := make([]apiRole, 0, len(views.AllExperience))
	for _, r := range views.AllExperience {
		experience = append(experience, apiRole{
			Title: r.Title, Org: r.Org, Period: r.Period,
			Location: r.Location, Bullets: strList(r.Bullets),
		})
	}

	education := make([]apiEducation, 0, len(views.AllEducation))
	for _, e := range views.AllEducation {
		education = append(education, apiEducation{
			School: e.School, Degree: e.Degree, Period: e.Period,
			Note: e.Note, Bullets: strList(e.Bullets),
		})
	}

	skills := make([]apiSkill, 0, len(views.AllSkills))
	for _, s := range views.AllSkills {
		skills = append(skills, apiSkill{Name: s.Name, Pct: s.Pct, Category: s.Category})
	}

	groups := make([]apiSkillGroup, 0, len(views.AllSkillGroups))
	for _, g := range views.AllSkillGroups {
		groups = append(groups, apiSkillGroup{Category: g.Category, Items: g.Items})
	}

	return apiContent{
		Name:        views.TheProfile.Name,
		Tagline:     views.TheProfile.Tagline,
		Location:    views.TheProfile.Location,
		Headline:    views.TheProfile.Headline,
		Drive:       strList(views.TheProfile.Drive),
		ResumeURL:   views.TheProfile.ResumeURL,
		Projects:    out,
		Contacts:    contacts,
		Experience:  experience,
		Education:   education,
		Skills:      skills,
		SkillGroups: groups,
		Now: apiNow{
			Building: strList(views.TheNow.Building),
			Learning: strList(views.TheNow.Learning),
			Reading:  strList(views.TheNow.Reading),
			Status:   strList(views.TheNow.Status),
		},
		Commit:    BuildCommit,
		Generated: time.Now().UTC().Format(time.RFC3339),
	}
}

// strList normalises a nil slice to an empty one. A nil slice encodes as JSON
// `null`, and the page's `Array.isArray(...)` guards would reject it and fall
// back to stale hardcoded content — so every list ships as `[]`.
func strList(in []string) []string {
	if in == nil {
		return []string{}
	}
	return in
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
