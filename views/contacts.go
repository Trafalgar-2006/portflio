package views

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/trafalgar-2006/ssh-portfolio/config"
)

type Contact struct {
	Icon  string
	Label string
	Value string
}

var AllContacts = []Contact{
	{Icon: "(@)", Label: "Email",    Value: "d.mohithakshay@gmail.com"},
	{Icon: "(~)", Label: "Website",  Value: "webcraftstudios.co.in"},
	{Icon: "(in)", Label: "LinkedIn", Value: "linkedin.com/in/dmohithakshay"},
	{Icon: "(gh)", Label: "GitHub",  Value: "github.com/trafalgar-2006"},
}

func loadContactsFromConfig() {
	if config.Loaded == nil || len(config.Loaded.Contacts) == 0 {
		return
	}
	var contacts []Contact
	for _, c := range config.Loaded.Contacts {
		contacts = append(contacts, Contact{
			Icon:  c.Icon,
			Label: c.Label,
			Value: c.Value,
		})
	}
	AllContacts = contacts
}

// padToWidth pads a styled string (with ANSI codes) to a target visual width using spaces.
func padToWidth(content string, targetWidth int) string {
	vis := lipgloss.Width(content)
	if vis >= targetWidth {
		return content
	}
	return content + strings.Repeat(" ", targetWidth-vis)
}

func RenderContacts(r *lipgloss.Renderer, width, height, contactsReveal, sshFlash int, copyMode bool, theme Theme) string {
	cyanStyle  := r.NewStyle().Foreground(lipgloss.Color(theme.Primary))
	dimStyle   := r.NewStyle().Foreground(lipgloss.Color(theme.Dim))
	dimMid     := r.NewStyle().Foreground(lipgloss.Color(theme.DimMid))
	whiteStyle := r.NewStyle().Foreground(lipgloss.Color(theme.Text))
	goldStyle  := r.NewStyle().Foreground(lipgloss.Color(theme.Accent)).Bold(true)
	linkStyle  := r.NewStyle().Foreground(lipgloss.Color(theme.Primary)).Underline(true)
	boxStyle   := r.NewStyle().Foreground(lipgloss.Color(theme.BoxBorder))
	divider    := dimStyle.Render("  " + strings.Repeat("─", 46))

	// Card dimensions — inner content width (between │ and │)
	const cardInner = 44 // visual chars between the two border pipes
	top    := boxStyle.Render("╭" + strings.Repeat("─", cardInner) + "╮")
	bottom := boxStyle.Render("╰" + strings.Repeat("─", cardInner) + "╯")
	lBdr   := boxStyle.Render("│")
	rBdr   := boxStyle.Render("│")

	// Build a card row: border + content fitted to cardInner + border.
	// Fit rather than pad — an over-long value would otherwise push the right
	// border out and break the rectangle.
	cardRow := func(content string) string {
		return "  " + lBdr + fitToWidth(r, " "+content, cardInner) + rBdr
	}

	// Colour by position in the list rather than by icon glyph, so changing an
	// icon in content.yaml can't silently drop a contact to an unthemed grey.
	iconPalette := []lipgloss.Color{
		lipgloss.Color(theme.Secondary),
		lipgloss.Color(theme.Success),
		lipgloss.Color(theme.Primary),
		lipgloss.Color(theme.Accent),
		lipgloss.Color(theme.Purple),
		lipgloss.Color(theme.Warning),
	}

	var b strings.Builder
	b.WriteString("\n")
	b.WriteString(" " + cyanStyle.Bold(true).Render("✦ Contacts") + "\n")
	b.WriteString(divider + "\n\n")
	b.WriteString("  " + dimMid.Render("Let's connect! Feel free to reach out.") + "\n\n")

	for i, c := range AllContacts {
		if i >= contactsReveal {
			b.WriteString("\n")
			continue
		}

		iconColor := iconPalette[i%len(iconPalette)]
		iconStyle := r.NewStyle().Foreground(iconColor).Bold(true)

		labelContent := iconStyle.Render(c.Icon) + "  " + goldStyle.Render(c.Label)
		valueContent := linkStyle.Render(c.Value)

		b.WriteString("  " + top + "\n")
		b.WriteString(cardRow(labelContent) + "\n")
		b.WriteString(cardRow(valueContent) + "\n")
		b.WriteString("  " + bottom + "\n\n")
	}

	b.WriteString(divider + "\n\n")

	// Copy-mode: plain-text dump for easy selection
	if copyMode {
		b.WriteString("  " + r.NewStyle().Foreground(lipgloss.Color(theme.Accent)).Bold(true).Render("── plain text (select to copy) ──") + "\n\n")
		for _, c := range AllContacts {
			b.WriteString("  " + c.Label + ": " + c.Value + "\n")
		}
		b.WriteString("\n  " + dimStyle.Render("[c] toggle · [esc] go back") + "\n")
	} else {
		sshLine := whiteStyle.Render("You're viewing this over ") + cyanStyle.Bold(true).Render("SSH") + whiteStyle.Render("!")
		if sshFlash > 0 && sshFlash%2 == 0 {
			sshLine = r.NewStyle().Foreground(lipgloss.Color("#FFFFFF")).Bold(true).Render("You're viewing this over SSH! ✦")
		}
		b.WriteString("  " + sshLine + "\n")
		b.WriteString("  " + dimStyle.Render("Built with Go + Bubbletea + Wish  ·  github.com/trafalgar-2006/portfolio") + "\n\n")
		b.WriteString("  " + dimStyle.Render("[c] copy-friendly view · ↑↓ scroll · [esc] go back") + "\n")
	}

	return b.String()
}
