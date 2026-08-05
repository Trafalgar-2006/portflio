package views

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// ─────────────────────────────────────────────────────────────────────────
// Content blocks
//
// One column, generous left margin, an explicit blank row between entries.
// No micro-columns, no tables, no boxes — text blocks can't collide because
// nothing ever shares a row.
// ─────────────────────────────────────────────────────────────────────────

// contentMargin is the left indent for every content block.
const contentMargin = 2

// bodyWidth is how wide prose is allowed to run before wrapping. Capped well
// below the terminal width so lines stay comfortably readable.
func bodyWidth(w int) int {
	b := w - contentMargin*2 - 4
	if b > 82 {
		b = 82
	}
	if b < 20 {
		b = 20
	}
	return b
}

func indent(s string) string { return strings.Repeat(" ", contentMargin) + s }

// bullet returns the soft status indicator for a project.
func bullet(status string) string {
	switch status {
	case "Live":
		return "●"
	case "WIP":
		return "◐"
	default:
		return "◇"
	}
}

// BlockProjects renders the project list as flat lines, ready to stream.
func BlockProjects(r *lipgloss.Renderer, w, cursor int, theme Theme) []string {
	titleS := r.NewStyle().Foreground(lipgloss.Color(theme.Text)).Bold(true)
	selS := r.NewStyle().Foreground(lipgloss.Color(theme.Primary)).Bold(true)
	descS := r.NewStyle().Foreground(lipgloss.Color(theme.DimMid))
	tagS := r.NewStyle().Foreground(lipgloss.Color(theme.Dim))
	hlS := r.NewStyle().Foreground(lipgloss.Color(theme.Accent))
	linkS := r.NewStyle().Foreground(lipgloss.Color(theme.Warning))

	projects := Projects()
	bw := bodyWidth(w)

	var out []string
	for i, p := range projects {
		dotCol := lipgloss.Color(theme.Dim)
		switch p.Status {
		case "Live":
			dotCol = lipgloss.Color(theme.Success)
		case "WIP":
			dotCol = lipgloss.Color(theme.Warning)
		case "Research":
			dotCol = lipgloss.Color(theme.Purple)
		}

		head := r.NewStyle().Foreground(dotCol).Render(bullet(p.Status)) + "  "
		if i == cursor {
			head += selS.Render(p.Title)
		} else {
			head += titleS.Render(p.Title)
		}
		if p.Highlight != "" {
			head += "   " + hlS.Render("⚡ "+p.Highlight)
		}
		out = append(out, indent(head))

		for _, line := range strings.Split(wrapWidth(p.Description, bw), "\n") {
			out = append(out, indent("   "+descS.Render(line)))
		}
		if len(p.Tags) > 0 {
			out = append(out, indent("   "+tagS.Render(strings.Join(p.Tags, "  ·  "))))
		}
		if p.GitHubURL != "" {
			out = append(out, indent("   "+linkS.Render("→ "+p.GitHubURL)))
		}

		// The explicit gap that stops entries from ever reading as one block.
		out = append(out, "")
	}
	if len(out) == 0 {
		out = append(out, indent(descS.Render("No projects configured.")))
	}
	return out
}

// BlockAbout renders the biography as a single column.
func BlockAbout(r *lipgloss.Renderer, w int, theme Theme) []string {
	h2 := r.NewStyle().Foreground(lipgloss.Color(theme.Primary)).Bold(true)
	roleS := r.NewStyle().Foreground(lipgloss.Color(theme.Text)).Bold(true)
	orgS := r.NewStyle().Foreground(lipgloss.Color(theme.Secondary))
	metaS := r.NewStyle().Foreground(lipgloss.Color(theme.Dim))
	bodyS := r.NewStyle().Foreground(lipgloss.Color(theme.DimMid))
	bw := bodyWidth(w)

	var out []string
	out = append(out, indent(h2.Render(TheProfile.Name)), "")
	for _, line := range strings.Split(wrapWidth(strings.Join(TheProfile.Drive, " "), bw), "\n") {
		out = append(out, indent(bodyS.Render(line)))
	}
	out = append(out, "", indent(h2.Render("Experience")), "")

	for _, role := range AllExperience {
		out = append(out, indent(roleS.Render(role.Title)))
		meta := role.Org
		if role.Period != "" {
			meta += "   " + role.Period
		}
		out = append(out, indent(orgS.Render(role.Org)+metaS.Render("   "+role.Period)))
		for _, b := range role.Bullets {
			for _, line := range strings.Split(wrapWidth(b, bw-3), "\n") {
				out = append(out, indent("   "+bodyS.Render(line)))
			}
		}
		out = append(out, "")
	}

	if len(AllEducation) > 0 {
		out = append(out, indent(h2.Render("Education")), "")
		for _, e := range AllEducation {
			out = append(out, indent(roleS.Render(e.School)))
			out = append(out, indent(orgS.Render(e.Degree)+metaS.Render("   "+e.Period)))
			out = append(out, "")
		}
	}
	return out
}

// BlockSkills renders proficiency as quiet bars, one per row.
func BlockSkills(r *lipgloss.Renderer, w int, theme Theme) []string {
	h2 := r.NewStyle().Foreground(lipgloss.Color(theme.Primary)).Bold(true)
	nameS := r.NewStyle().Foreground(lipgloss.Color(theme.DimMid))
	dimS := r.NewStyle().Foreground(lipgloss.Color(theme.Dim))

	colour := map[string]lipgloss.Color{
		"lang":  lipgloss.Color(theme.Primary),
		"ml":    lipgloss.Color(theme.Secondary),
		"frame": lipgloss.Color(theme.Purple),
		"infra": lipgloss.Color(theme.Success),
	}

	out := []string{indent(h2.Render("Toolkit")), ""}
	const barW = 24
	for _, s := range AllSkills {
		col, ok := colour[s.Category]
		if !ok {
			col = lipgloss.Color(theme.Primary)
		}
		filled := clampInt(s.Pct*barW/100, 0, barW)
		bar := r.NewStyle().Foreground(col).Render(strings.Repeat("━", filled)) +
			dimS.Render(strings.Repeat("━", barW-filled))
		out = append(out, indent(padToWidth(nameS.Render(s.Name), 14)+bar))
	}
	out = append(out, "")
	return out
}

// BlockResume renders the resume as one column.
func BlockResume(r *lipgloss.Renderer, w int, theme Theme) []string {
	h2 := r.NewStyle().Foreground(lipgloss.Color(theme.Primary)).Bold(true)
	bodyS := r.NewStyle().Foreground(lipgloss.Color(theme.DimMid))
	linkS := r.NewStyle().Foreground(lipgloss.Color(theme.Warning))
	catS := r.NewStyle().Foreground(lipgloss.Color(theme.Text)).Bold(true)

	out := BlockAbout(r, w, theme)
	out = append(out, BlockSkills(r, w, theme)...)

	if len(AllSkillGroups) > 0 {
		out = append(out, indent(h2.Render("Stack")), "")
		for _, g := range AllSkillGroups {
			out = append(out, indent(catS.Render(g.Category)))
			out = append(out, indent("   "+bodyS.Render(g.Items)), "")
		}
	}
	out = append(out, indent(h2.Render("Download")), "")
	out = append(out, indent(linkS.Render(TheProfile.ResumeURL)), "")
	return out
}

// BlockContacts renders contacts as one column.
func BlockContacts(r *lipgloss.Renderer, w int, copyMode bool, theme Theme) []string {
	h2 := r.NewStyle().Foreground(lipgloss.Color(theme.Primary)).Bold(true)
	labelS := r.NewStyle().Foreground(lipgloss.Color(theme.Dim))
	valS := r.NewStyle().Foreground(lipgloss.Color(theme.Text))
	hintS := r.NewStyle().Foreground(lipgloss.Color(theme.DimMid))

	out := []string{indent(h2.Render("Contact")), ""}
	if copyMode {
		out = append(out, indent(hintS.Render("plain text — select to copy")), "")
		for _, c := range AllContacts {
			out = append(out, indent(c.Label+": "+c.Value))
		}
		out = append(out, "", indent(hintS.Render("[c] back to styled view")))
		return out
	}
	for _, c := range AllContacts {
		out = append(out, indent(padToWidth(labelS.Render(c.Label), 12)+valS.Render(c.Value)))
		out = append(out, "")
	}
	out = append(out, indent(hintS.Render("[c] copy-friendly view")))
	return out
}

// BlockNow renders the /now page as one column.
func BlockNow(r *lipgloss.Renderer, w int, updated string, theme Theme) []string {
	h2 := r.NewStyle().Foreground(lipgloss.Color(theme.Primary)).Bold(true)
	labelS := r.NewStyle().Foreground(lipgloss.Color(theme.Dim))
	bodyS := r.NewStyle().Foreground(lipgloss.Color(theme.DimMid))
	dotS := r.NewStyle().Foreground(lipgloss.Color(theme.Success))

	section := func(out []string, title string, items []string) []string {
		if len(items) == 0 {
			return out
		}
		out = append(out, indent(labelS.Render(title)), "")
		for _, it := range items {
			out = append(out, indent("  "+dotS.Render("▸")+"  "+bodyS.Render(it)))
		}
		return append(out, "")
	}

	out := []string{indent(h2.Render("Now")), ""}
	out = section(out, "BUILDING", TheNow.Building)
	out = section(out, "LEARNING", TheNow.Learning)
	out = section(out, "READING", TheNow.Reading)
	out = section(out, "STATUS", TheNow.Status)
	if updated != "" {
		out = append(out, indent(labelS.Render("last updated "+updated)))
	}
	return out
}
