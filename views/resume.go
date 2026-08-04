package views

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func RenderResume(r *lipgloss.Renderer, width, height int, theme Theme) string {
	cyanStyle    := r.NewStyle().Foreground(lipgloss.Color(theme.Primary))
	goldStyle    := r.NewStyle().Foreground(lipgloss.Color(theme.Accent))
	magentaStyle := r.NewStyle().Foreground(lipgloss.Color(theme.Secondary))
	greenStyle   := r.NewStyle().Foreground(lipgloss.Color(theme.Success))
	purpleStyle  := r.NewStyle().Foreground(lipgloss.Color(theme.Purple))
	dimStyle     := r.NewStyle().Foreground(lipgloss.Color(theme.DimMid))
	whiteStyle   := r.NewStyle().Foreground(lipgloss.Color(theme.Text))
	orangeStyle  := r.NewStyle().Foreground(lipgloss.Color(theme.Warning))
	divider      := dimStyle.Render("  " + strings.Repeat("─", 50))

	var b strings.Builder
	b.WriteString("\n")

	// ── Header ──────────────────────────────────────────────────────────────
	b.WriteString("  " + cyanStyle.Bold(true).Render(strings.ToUpper(TheProfile.Name)) + "\n")
	b.WriteString("  " + magentaStyle.Render(TheProfile.Headline) + "\n")

	// Contact line, built from the same list the Contacts tab uses.
	var bits []string
	for _, c := range AllContacts {
		bits = append(bits, c.Value)
	}
	if len(bits) > 0 {
		b.WriteString("  " + dimStyle.Render(strings.Join(bits, "  ·  ")) + "\n")
	}
	b.WriteString(divider + "\n\n")

	// ── Education ────────────────────────────────────────────────────────────
	if len(AllEducation) > 0 {
		b.WriteString("  " + goldStyle.Bold(true).Render("◆ EDUCATION") + "\n\n")
		for _, e := range AllEducation {
			b.WriteString("  " + whiteStyle.Bold(true).Render(e.School) + "\n")
			b.WriteString("  " + cyanStyle.Render(e.Degree) + "\n")
			b.WriteString("  " + dimStyle.Render(e.Period) + "\n\n")
		}
		b.WriteString(divider + "\n\n")
	}

	// ── Experience ───────────────────────────────────────────────────────────
	if len(AllExperience) > 0 {
		b.WriteString("  " + goldStyle.Bold(true).Render("◆ EXPERIENCE") + "\n\n")
		for _, role := range AllExperience {
			b.WriteString("  " + whiteStyle.Bold(true).Render(role.Title) + "\n")
			line := "  " + cyanStyle.Render(role.Org)
			if role.Period != "" {
				line += "  " + dimStyle.Render(role.Period)
			}
			b.WriteString(line + "\n")
			for _, bullet := range role.Bullets {
				b.WriteString("  " + greenStyle.Render("▸ ") + dimStyle.Render(bullet) + "\n")
			}
			b.WriteString("\n")
		}
		b.WriteString(divider + "\n\n")
	}

	// ── Skills ───────────────────────────────────────────────────────────────
	if len(AllSkillGroups) > 0 {
		b.WriteString("  " + goldStyle.Bold(true).Render("◆ SKILLS") + "\n\n")
		for _, g := range AllSkillGroups {
			b.WriteString("  " + purpleStyle.Bold(true).Render(g.Category) + "\n")
			b.WriteString("  " + dimStyle.Render("  "+g.Items) + "\n\n")
		}
		b.WriteString(divider + "\n\n")
	}

	// ── Key Projects ──────────────────────────────────────────────────────────
	// Drawn from the live project list, so the resume can't drift from Projects.
	keyProjects := Projects()
	if len(keyProjects) > 0 {
		b.WriteString("  " + goldStyle.Bold(true).Render("◆ KEY PROJECTS") + "\n\n")
		shown := 0
		for _, p := range keyProjects {
			if shown >= 4 {
				break
			}
			head := "  " + whiteStyle.Bold(true).Render(p.Title)
			if tech := strings.Join(p.Tags, " · "); tech != "" {
				head += "  " + dimStyle.Render("("+tech+")")
			}
			b.WriteString(head + "\n")
			// One-sentence summary keeps the resume tight.
			desc := p.Description
			if i := strings.Index(desc, ". "); i > 0 {
				desc = desc[:i+1]
			}
			for _, line := range strings.Split(wrapWidth(desc, 66), "\n") {
				b.WriteString("    " + dimStyle.Render(line) + "\n")
			}
			b.WriteString("\n")
			shown++
		}
		b.WriteString(divider + "\n\n")
	}

	// ── Download ─────────────────────────────────────────────────────────────
	b.WriteString("  " + goldStyle.Bold(true).Render("◆ DOWNLOAD RESUME (PDF)") + "\n\n")
	b.WriteString("  " + orangeStyle.Render(TheProfile.ResumeURL) + "\n\n")

	b.WriteString(divider + "\n\n")
	b.WriteString("  " + dimStyle.Render("[↑↓/jk scroll · PgUp/PgDn page · gg/G top-bottom · esc back]") + "\n")

	return b.String()
}
