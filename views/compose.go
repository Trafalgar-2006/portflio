package views

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// ─────────────────────────────────────────────────────────────────────────
// Screen composition
//
//	┌ header  (portrait + identity + nav, unboxed)
//	├ ─────── rule
//	│ content (single column, generous margins)
//	├ ─────── rule
//	└ footer  (heartbeat left, telemetry right)
//
// Exactly two rules in the whole interface. Everything else is space.
// ─────────────────────────────────────────────────────────────────────────

// Telemetry is the passive live data, kept out of the content entirely.
type Telemetry struct {
	Latency int
	Online  int64
	Session string
	Uptime  string
}

// ScreenState is one full frame.
type ScreenState struct {
	Header    string // pre-rendered hero
	Content   string // pre-rendered body
	Scroll    int    // first content row to show
	Beat      int    // heartbeat phase, on its own slow ticker
	Telemetry Telemetry
	Hint      string // e.g. "? help   / search"
}

// ContentHeight reports how many rows the content pane gets for a given
// terminal and header. The model needs this to clamp scrolling without
// rendering the frame twice.
func ContentHeight(h, headerRows int) int {
	// blank + header + blank + rule + blank … blank + rule + footer
	chrome := 1 + headerRows + 1 + 1 + 1 + 1 + 1 + 1
	if n := h - chrome; n > 0 {
		return n
	}
	return 0
}

// rule draws a full-width hairline.
func rule(r *lipgloss.Renderer, w int, theme Theme) string {
	if w <= 0 {
		return ""
	}
	return r.NewStyle().Foreground(lipgloss.Color(theme.Dim)).Render(strings.Repeat("─", w))
}

// footer is the persistent bottom line: pulse on the left, telemetry on the
// right, both deliberately quiet.
func footer(r *lipgloss.Renderer, w int, st ScreenState, theme Theme) string {
	if w <= 0 {
		return ""
	}
	beatS := r.NewStyle().Foreground(lipgloss.Color(theme.Success))
	hintS := r.NewStyle().Foreground(lipgloss.Color(theme.DimMid))
	// Telemetry sits a full step below the hint in contrast: it is there to be
	// glanced at, never read.
	teleS := r.NewStyle().Foreground(lipgloss.Color(theme.Dim))

	left := " " + beatS.Render(Heartbeat(st.Beat))
	if st.Hint != "" {
		left += "  " + hintS.Render(st.Hint)
	}

	var parts []string
	t := st.Telemetry
	if t.Latency > 0 {
		parts = append(parts, itoa(t.Latency)+"ms")
	}
	if t.Online > 0 {
		parts = append(parts, itoa(int(t.Online))+" online")
	}
	if t.Session != "" {
		parts = append(parts, t.Session)
	}
	if t.Uptime != "" {
		parts = append(parts, t.Uptime)
	}
	right := teleS.Render(strings.Join(parts, "  ·  ") + " ")

	gap := w - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 1 {
		// No room for both: telemetry is the first thing to go.
		return fitToWidth(r, left, w)
	}
	return left + strings.Repeat(" ", gap) + right
}

// Compose assembles a full frame of exactly h rows and w columns.
//
// The content region absorbs whatever height is left over, which is what
// creates the deep negative space: a short page simply sits in a large empty
// field rather than being stretched or boxed to fill it.
func Compose(r *lipgloss.Renderer, w, h int, st ScreenState, theme Theme) string {
	if w <= 0 || h <= 0 {
		return ""
	}

	headerLines := splitNonEmptyTail(st.Header)
	contentLines := splitNonEmptyTail(st.Content)

	// Reserve: blank, header, blank, rule, blank … rule, footer.
	const (
		padTop      = 1
		padAfterHdr = 1
		padAfterRl  = 1
		padBeforeRl = 1
	)
	chrome := padTop + len(headerLines) + padAfterHdr + 1 + padAfterRl + padBeforeRl + 1 + 1

	contentH := h - chrome
	if contentH < 0 {
		contentH = 0
	}

	var rows []string
	add := func(lines ...string) { rows = append(rows, lines...) }

	add(strings.Repeat("\n", padTop-1))
	if padTop > 0 {
		rows = rows[:len(rows)-1]
		for i := 0; i < padTop; i++ {
			add("")
		}
	}

	// Header — dropped entirely when the terminal is too short for it.
	if contentH > 2 {
		add(headerLines...)
		for i := 0; i < padAfterHdr; i++ {
			add("")
		}
		add(rule(r, w, theme))
		for i := 0; i < padAfterRl; i++ {
			add("")
		}
	} else {
		contentH = h - 3
		if contentH < 0 {
			contentH = 0
		}
	}

	// Content, windowed by the scroll offset. The portrait is deliberately
	// large, so on most terminals the body is taller than the pane — it
	// scrolls rather than being truncated.
	scroll := st.Scroll
	if max := len(contentLines) - contentH; scroll > max {
		scroll = max
	}
	if scroll < 0 {
		scroll = 0
	}
	shown := contentLines[minInt(scroll, len(contentLines)):]
	if len(shown) > contentH {
		shown = shown[:contentH]
	}
	add(shown...)
	for i := len(shown); i < contentH; i++ {
		add("") // the negative space
	}

	// A quiet mark when there's more below, placed in the last content row so
	// it never costs a line of its own.
	if len(contentLines) > scroll+contentH && contentH > 0 && len(rows) > 0 {
		more := r.NewStyle().Foreground(lipgloss.Color(theme.Dim)).
			Render("  ↓ " + itoa(len(contentLines)-scroll-contentH) + " more")
		rows[len(rows)-1] = more
	}

	for i := 0; i < padBeforeRl; i++ {
		add("")
	}
	add(rule(r, w, theme))
	add(footer(r, w, st, theme))

	// Pad or clip to exactly h rows.
	for len(rows) < h {
		rows = append(rows, "")
	}
	if len(rows) > h {
		// Too tall: drop from the middle so the footer survives. On a terminal
		// with fewer rows than the chrome needs there is nothing to preserve,
		// so just take the top — computing an offset here is what produced a
		// negative slice bound on a 1-row window.
		const keepBottom = 2
		if h > keepBottom {
			rows = append(rows[:h-keepBottom], rows[len(rows)-keepBottom:]...)
		}
		rows = rows[:h]
	}

	// Final guarantee: no row may exceed the terminal width. Enforcing it here
	// rather than in every producer means a long name, a wide tag list or an
	// un-wrappable URL can never push the frame sideways on a narrow client.
	clip := r.NewStyle().MaxWidth(w)
	for i, row := range rows {
		if lipgloss.Width(row) > w {
			rows[i] = clip.Render(row)
		}
	}
	return strings.Join(rows, "\n")
}

// splitNonEmptyTail splits a block into lines, discarding a single trailing
// empty line left by a final newline.
func splitNonEmptyTail(s string) []string {
	if s == "" {
		return nil
	}
	lines := strings.Split(s, "\n")
	if n := len(lines); n > 0 && strings.TrimSpace(lines[n-1]) == "" {
		lines = lines[:n-1]
	}
	return lines
}
