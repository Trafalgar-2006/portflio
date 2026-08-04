package views

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// RenderNow draws the /now page. lastUpdated is derived from the build date so
// the freshness stamp can't drift out of date the way a hardcoded one does.
func RenderNow(r *lipgloss.Renderer, width, height int, lastUpdated string, theme Theme) string {
	cyanStyle    := r.NewStyle().Foreground(lipgloss.Color(theme.Primary))
	goldStyle    := r.NewStyle().Foreground(lipgloss.Color(theme.Accent)).Bold(true)
	dimStyle     := r.NewStyle().Foreground(lipgloss.Color(theme.Dim))
	dimMidStyle  := r.NewStyle().Foreground(lipgloss.Color(theme.DimMid))
	magentaStyle := r.NewStyle().Foreground(lipgloss.Color(theme.Secondary))
	greenStyle   := r.NewStyle().Foreground(lipgloss.Color(theme.Success))
	boxStyle     := r.NewStyle().Foreground(lipgloss.Color(theme.BoxBorder))
	hintStyle    := r.NewStyle().Foreground(lipgloss.Color(theme.VeryDim)).Italic(true)
	divider      := dimStyle.Render("  " + strings.Repeat("─", 50))

	var b strings.Builder
	b.WriteString("\n")
	b.WriteString(" " + cyanStyle.Bold(true).Render("✦ /now") + "\n")
	b.WriteString(divider + "\n\n")
	b.WriteString("  " + dimMidStyle.Italic(true).Render("A live snapshot of what I'm building and focused on.") + "\n\n")

	// Size the box to its longest entry (+ bullet + padding) so content.yaml
	// items aren't silently truncated, then clamp to the terminal.
	boxW := 52
	for _, list := range [][]string{TheNow.Building, TheNow.Learning} {
		for _, it := range list {
			if need := lipgloss.Width(it) + 4; need > boxW {
				boxW = need
			}
		}
	}
	if width-6 < boxW { boxW = width - 6 }
	if boxW < minBoxWidth { boxW = minBoxWidth } // guard: negative Repeat on tiny terminals
	bTop := "  " + boxStyle.Render("╭"+strings.Repeat("─", boxW)+"╮")
	bBot := "  " + boxStyle.Render("╰"+strings.Repeat("─", boxW)+"╯")
	bRow := func(s string) string {
		// Fit, not just pad — long entries would push the right border out.
		return "  " + boxStyle.Render("│") + fitToWidth(r, " "+s, boxW) + boxStyle.Render("│")
	}

	// Boxed list sections, driven entirely by content.yaml.
	section := func(title string, items []string) {
		if len(items) == 0 {
			return
		}
		b.WriteString("  " + goldStyle.Render("◆ "+title) + "\n")
		b.WriteString(bTop + "\n")
		for _, it := range items {
			b.WriteString(bRow(greenStyle.Render("▸ ") + dimStyle.Render(it)) + "\n")
		}
		b.WriteString(bBot + "\n\n")
	}

	section("Currently Building", TheNow.Building)
	section("Currently Learning", TheNow.Learning)

	if len(TheNow.Reading) > 0 {
		b.WriteString("  " + goldStyle.Render("◆ Reading") + "\n")
		for _, it := range TheNow.Reading {
			b.WriteString("  " + dimStyle.Render("  "+it) + "\n")
		}
		b.WriteString("\n")
	}

	if len(TheNow.Status) > 0 {
		b.WriteString("  " + goldStyle.Render("◆ Status") + "\n")
		for i, it := range TheNow.Status {
			if i == 0 {
				b.WriteString("  " + magentaStyle.Render(it) + "\n")
			} else {
				b.WriteString("  " + greenStyle.Render(it) + "\n")
			}
		}
		b.WriteString("\n")
	}

	b.WriteString("  " + boxStyle.Render(strings.Repeat("─", 50)) + "\n")
	if lastUpdated != "" {
		b.WriteString("  " + dimMidStyle.Italic(true).Render("last updated: "+lastUpdated) + "\n\n")
	} else {
		b.WriteString("\n")
	}
	b.WriteString("  " + hintStyle.Render("[↑↓/jk scroll · PgUp/PgDn page · gg/G top-bottom · esc back]") + "\n")

	return b.String()
}
