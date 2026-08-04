package views

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// greeting returns a time-of-day line in IST.
func greeting() string {
	ist := time.FixedZone("IST", 5*60*60+30*60)
	switch hour := time.Now().In(ist).Hour(); {
	case hour >= 2 && hour < 5:
		return "you're up late. so am I, probably."
	case hour >= 5 && hour < 9:
		return "early start. respect."
	case hour >= 9 && hour < 18:
		return "currently probably in class or debugging something."
	case hour >= 18 && hour < 22:
		return "golden hours. this is when the best code gets written."
	default:
		return "late night build session energy in here."
	}
}

func RenderAbout(r *lipgloss.Renderer, width, height int, theme Theme) string {
	cyanStyle    := r.NewStyle().Foreground(lipgloss.Color(theme.Primary))
	dimStyle     := r.NewStyle().Foreground(lipgloss.Color(theme.DimMid))
	dimDarkStyle := r.NewStyle().Foreground(lipgloss.Color(theme.Dim))
	whiteStyle   := r.NewStyle().Foreground(lipgloss.Color(theme.Text))
	magentaStyle := r.NewStyle().Foreground(lipgloss.Color(theme.Secondary))
	goldStyle    := r.NewStyle().Foreground(lipgloss.Color(theme.Accent))
	greenStyle   := r.NewStyle().Foreground(lipgloss.Color(theme.Success))
	purpleStyle  := r.NewStyle().Foreground(lipgloss.Color(theme.Purple))
	boxStyle     := r.NewStyle().Foreground(lipgloss.Color(theme.BoxBorder))
	divider      := dimDarkStyle.Render("  " + strings.Repeat("─", 50))

	boxW := 56
	if width < boxW+6 { boxW = width - 6 }
	if boxW < minBoxWidth { boxW = minBoxWidth } // guard: negative Repeat on tiny terminals
	secTop := func(label string) string {
		inner := " " + label + " "
		// Measure display columns, not bytes — label carries ANSI escapes.
		pad := boxW - 2 - lipgloss.Width(inner)
		if pad < 0 { pad = 0 }
		return "  " + boxStyle.Render("╭") + inner + boxStyle.Render(strings.Repeat("─", pad)+"╮")
	}
	secBot := "  " + boxStyle.Render("╰"+strings.Repeat("─", boxW-2)+"╯")
	secRow := func(s string) string {
		// Fit, not just pad — long bullets would push the right border out.
		return "  " + boxStyle.Render("│") + fitToWidth(r, s, boxW-2) + boxStyle.Render("│")
	}
	secBlank := func() string { return secRow("") }

	var b strings.Builder
	b.WriteString("\n")
	b.WriteString("  " + dimStyle.Italic(true).Render(greeting()) + "\n\n")

	b.WriteString(" " + cyanStyle.Bold(true).Render("✦ About") + "\n")
	b.WriteString(divider + "\n\n")

	// Name & tagline
	b.WriteString("  " + goldStyle.Bold(true).Render(TheProfile.Name) + "\n")
	b.WriteString("  " + magentaStyle.Italic(true).Render(TheProfile.Tagline) + "\n")
	b.WriteString("  " + dimStyle.Render(TheProfile.Location) + "\n\n")

	// ── Education ──────────────────────────────────────────────────────
	if len(AllEducation) > 0 {
		b.WriteString(secTop(cyanStyle.Bold(true).Render("◆ Education")) + "\n")
		b.WriteString(secBlank() + "\n")
		for _, e := range AllEducation {
			b.WriteString(secRow("  "+whiteStyle.Bold(true).Render(e.School)) + "\n")
			b.WriteString(secRow("  "+whiteStyle.Render(e.Degree)) + "\n")
			meta := e.Period
			if e.Note != "" {
				meta += "  ·  " + e.Note
			}
			b.WriteString(secRow("  "+dimStyle.Render(meta)) + "\n")
			for _, bullet := range e.Bullets {
				b.WriteString(secRow("  "+greenStyle.Render("▸ ")+dimStyle.Render(bullet)) + "\n")
			}
		}
		b.WriteString(secBlank() + "\n")
		b.WriteString(secBot + "\n\n")
	}

	// ── Experience ─────────────────────────────────────────────────────
	if len(AllExperience) > 0 {
		b.WriteString(secTop(cyanStyle.Bold(true).Render("◆ Experience")) + "\n")
		b.WriteString(secBlank() + "\n")
		for _, role := range AllExperience {
			meta := role.Period
			if role.Location != "" {
				meta += "  ·  " + role.Location
			}
			b.WriteString(secRow("  "+goldStyle.Bold(true).Render(role.Title)) + "\n")
			b.WriteString(secRow("  "+magentaStyle.Render(role.Org)+"  "+dimStyle.Render(meta)) + "\n")
			for _, bullet := range role.Bullets {
				b.WriteString(secRow("  "+greenStyle.Render("▸ ")+dimStyle.Render(bullet)) + "\n")
			}
			b.WriteString(secBlank() + "\n")
		}
		b.WriteString(secBot + "\n\n")
	}

	// ── Skills ─────────────────────────────────────────────────────────
	if len(AllSkills) > 0 {
		b.WriteString(secTop(cyanStyle.Bold(true).Render("◆ Skills")) + "\n")
		b.WriteString(secBlank() + "\n")

		barColor := map[string]lipgloss.Color{
			"lang":  lipgloss.Color(theme.Primary),
			"ml":    lipgloss.Color(theme.Secondary),
			"frame": lipgloss.Color(theme.Purple),
			"infra": lipgloss.Color(theme.Success),
		}

		const barTotal = 18
		for _, s := range AllSkills {
			col, ok := barColor[s.Category]
			if !ok {
				col = lipgloss.Color(theme.Primary)
			}
			filled := (s.Pct * barTotal) / 100
			if filled > barTotal { filled = barTotal }
			if filled < 0 { filled = 0 }
			bar := r.NewStyle().Foreground(col).Render(strings.Repeat("█", filled)) +
				dimDarkStyle.Render(strings.Repeat("░", barTotal-filled))
			row := " " + dimStyle.Render(fmt.Sprintf("%-12s", s.Name)) + "  " + bar +
				"  " + dimDarkStyle.Render(fmt.Sprintf("%3d%%", s.Pct))
			b.WriteString(secRow(row) + "\n")
		}
		b.WriteString(secBlank() + "\n")
		b.WriteString(secBot + "\n\n")
	}

	// ── What drives me ─────────────────────────────────────────────────
	if len(TheProfile.Drive) > 0 {
		b.WriteString("  " + cyanStyle.Bold(true).Render("◆ What drives me") + "\n")
		for _, line := range TheProfile.Drive {
			b.WriteString("  " + dimStyle.Render(line) + "\n")
		}
		b.WriteString("\n")
	}

	b.WriteString("  " + dimDarkStyle.Render("Resume →") + "\n")
	b.WriteString("  " + purpleStyle.Render(TheProfile.ResumeURL) + "\n\n")

	b.WriteString(divider + "\n\n")
	b.WriteString("  " + dimDarkStyle.Render("[↑↓/jk scroll · PgUp/PgDn page · gg/G top-bottom · esc back]") + "\n")

	return b.String()
}
