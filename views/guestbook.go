package views

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// GuestEntry is one message on the shared wall.
type GuestEntry struct {
	Name    string    `json:"name"`
	Message string    `json:"message"`
	At      time.Time `json:"at"`
	Session string    `json:"session,omitempty"`
}

// relativeTime renders a timestamp as "3m ago" style text.
func relativeTime(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	case d < 30*24*time.Hour:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	default:
		return t.Format("2 Jan 2006")
	}
}

// GuestbookInput is the compose-box state owned by the model.
type GuestbookInput struct {
	Active     bool   // composing rather than reading
	Field      int    // 0 = name, 1 = message
	Name       string
	Message    string
	Err        string
	JustPosted bool
}

// RenderGuestbook draws the shared message wall plus the compose box.
func RenderGuestbook(r *lipgloss.Renderer, width, height int, entries []GuestEntry,
	in GuestbookInput, online int, blink bool, theme Theme) string {

	cyanS := r.NewStyle().Foreground(lipgloss.Color(theme.Primary))
	goldS := r.NewStyle().Foreground(lipgloss.Color(theme.Accent)).Bold(true)
	dimS := r.NewStyle().Foreground(lipgloss.Color(theme.Dim))
	midS := r.NewStyle().Foreground(lipgloss.Color(theme.DimMid))
	textS := r.NewStyle().Foreground(lipgloss.Color(theme.Text))
	boxS := r.NewStyle().Foreground(lipgloss.Color(theme.BoxBorder))
	okS := r.NewStyle().Foreground(lipgloss.Color(theme.Success))
	errS := r.NewStyle().Foreground(lipgloss.Color(theme.Warning))
	hintS := r.NewStyle().Foreground(lipgloss.Color(theme.VeryDim)).Italic(true)

	inner := width - 6
	if inner < minBoxWidth {
		inner = minBoxWidth
	}
	if inner > 92 {
		inner = 92
	}

	var b strings.Builder
	b.WriteString("\n")
	b.WriteString(" " + cyanS.Bold(true).Render("✦ Guestbook") + "  " +
		midS.Render(fmt.Sprintf("%d messages", len(entries))) + "  " +
		okS.Render(fmt.Sprintf("· %d online", online)) + "\n")
	b.WriteString(dimS.Render("  "+strings.Repeat("─", inner)) + "\n\n")

	if len(entries) == 0 {
		b.WriteString("  " + midS.Render("No messages yet — be the first to sign it.") + "\n\n")
	}

	// Newest last, so the wall reads like a chat log.
	for _, e := range entries {
		head := goldS.Render(e.Name) + dimS.Render("  ·  "+relativeTime(e.At))
		b.WriteString("  " + head + "\n")
		for _, line := range strings.Split(wrapWidth(e.Message, inner-4), "\n") {
			b.WriteString("    " + textS.Render(line) + "\n")
		}
		b.WriteString("\n")
	}

	b.WriteString(dimS.Render("  "+strings.Repeat("─", inner)) + "\n")

	// ── Compose box ───────────────────────────────────────────────────
	if in.Active {
		caret := "█"
		if !blink {
			caret = " "
		}
		nameVal, msgVal := in.Name, in.Message
		if in.Field == 0 {
			nameVal += caret
		} else {
			msgVal += caret
		}

		label := func(active bool, s string) string {
			if active {
				return cyanS.Bold(true).Render(s)
			}
			return midS.Render(s)
		}

		b.WriteString("  " + boxS.Render("╭"+strings.Repeat("─", inner-2)+"╮") + "\n")
		b.WriteString("  " + boxS.Render("│") +
			fitToWidth(r, " "+label(in.Field == 0, "name ")+textS.Render(nameVal), inner-2) +
			boxS.Render("│") + "\n")
		b.WriteString("  " + boxS.Render("│") +
			fitToWidth(r, " "+label(in.Field == 1, "note ")+textS.Render(msgVal), inner-2) +
			boxS.Render("│") + "\n")
		b.WriteString("  " + boxS.Render("╰"+strings.Repeat("─", inner-2)+"╯") + "\n")

		if in.Err != "" {
			b.WriteString("  " + errS.Render("! "+in.Err) + "\n")
		}
		b.WriteString("  " + hintS.Render("tab switch field · enter post · esc cancel") + "\n")
	} else {
		if in.JustPosted {
			b.WriteString("  " + okS.Render("✓ posted — thanks for signing!") + "\n")
		}
		b.WriteString("  " + hintS.Render("[i] write a message · ↑↓ scroll · [esc] back") + "\n")
	}

	return b.String()
}
