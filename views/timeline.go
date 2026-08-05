package views

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// Commit is one entry in the project's history.
type Commit struct {
	SHA     string
	Message string
	Author  string
	At      time.Time
}

// timelineMu guards TheCommits, which a background fetch writes while
// sessions render it.
var (
	timelineMu sync.RWMutex
	theCommits []Commit
)

// SetCommits replaces the timeline history.
func SetCommits(cs []Commit) {
	timelineMu.Lock()
	defer timelineMu.Unlock()
	theCommits = cs
}

// Commits returns a snapshot of the timeline history, newest first.
func Commits() []Commit {
	timelineMu.RLock()
	defer timelineMu.RUnlock()
	out := make([]Commit, len(theCommits))
	copy(out, theCommits)
	return out
}

// RenderTimeline draws the "time travel" view: a scrubber across the project's
// commit history, with the selected commit expanded. cursor 0 is the newest.
func RenderTimeline(r *lipgloss.Renderer, width, height, cursor int, commits []Commit, theme Theme) string {
	cyanS := r.NewStyle().Foreground(lipgloss.Color(theme.Primary))
	goldS := r.NewStyle().Foreground(lipgloss.Color(theme.Accent)).Bold(true)
	dimS := r.NewStyle().Foreground(lipgloss.Color(theme.Dim))
	midS := r.NewStyle().Foreground(lipgloss.Color(theme.DimMid))
	textS := r.NewStyle().Foreground(lipgloss.Color(theme.Text))
	boxS := r.NewStyle().Foreground(lipgloss.Color(theme.BoxBorder))
	hintS := r.NewStyle().Foreground(lipgloss.Color(theme.VeryDim)).Italic(true)

	inner := width - 6
	if inner < minBoxWidth {
		inner = minBoxWidth
	}
	if inner > 88 {
		inner = 88
	}

	var b strings.Builder
	b.WriteString("\n")
	b.WriteString(" " + cyanS.Bold(true).Render("✦ Time travel") + "  " +
		midS.Render("this portfolio's own history") + "\n")
	b.WriteString(dimS.Render("  "+strings.Repeat("─", inner)) + "\n\n")

	if len(commits) == 0 {
		b.WriteString("  " + midS.Render("Fetching history from GitHub…") + "\n\n")
		b.WriteString("  " + hintS.Render("[esc] back") + "\n")
		return b.String()
	}

	if cursor < 0 {
		cursor = 0
	}
	if cursor >= len(commits) {
		cursor = len(commits) - 1
	}

	// ── Scrubber ──────────────────────────────────────────────────────
	// One cell per commit where they fit, otherwise a proportional bar.
	track := inner - 4
	if track < 4 {
		track = 4
	}
	pos := 0
	if len(commits) > 1 {
		pos = cursor * (track - 1) / (len(commits) - 1)
	}
	var scrub strings.Builder
	for i := 0; i < track; i++ {
		switch {
		case i == pos:
			scrub.WriteString(goldS.Render("◉"))
		case i < pos:
			scrub.WriteString(cyanS.Render("─"))
		default:
			scrub.WriteString(dimS.Render("─"))
		}
	}
	b.WriteString("  " + scrub.String() + "\n")

	newest, oldest := commits[0], commits[len(commits)-1]
	b.WriteString("  " + dimS.Render(fmt.Sprintf("%-*s%s",
		track/2, oldest.At.Format("Jan 2006"), newest.At.Format("Jan 2006"))) + "\n\n")

	// ── Selected commit ───────────────────────────────────────────────
	c := commits[cursor]
	b.WriteString("  " + boxS.Render("╭"+strings.Repeat("─", inner-2)+"╮") + "\n")
	row := func(s string) string {
		return "  " + boxS.Render("│") + fitToWidth(r, " "+s, inner-2) + boxS.Render("│")
	}
	b.WriteString(row(goldS.Render(shortSHA(c.SHA))+"  "+midS.Render(relativeTime(c.At))) + "\n")
	b.WriteString(row("") + "\n")
	for _, line := range strings.Split(wrapWidth(c.Message, inner-6), "\n") {
		b.WriteString(row(textS.Render(line)) + "\n")
	}
	b.WriteString(row("") + "\n")
	b.WriteString(row(dimS.Render(c.At.Format("Mon 2 Jan 2006, 15:04"))) + "\n")
	b.WriteString("  " + boxS.Render("╰"+strings.Repeat("─", inner-2)+"╯") + "\n\n")

	// ── Neighbouring commits for context ──────────────────────────────
	b.WriteString("  " + midS.Render(fmt.Sprintf("commit %d of %d", cursor+1, len(commits))) + "\n\n")
	for i := cursor - 2; i <= cursor+2; i++ {
		if i < 0 || i >= len(commits) {
			continue
		}
		marker := "  "
		style := dimS
		if i == cursor {
			marker = goldS.Render("▸ ")
			style = textS
		}
		msg := firstLine(commits[i].Message)
		if len([]rune(msg)) > inner-24 {
			msg = string([]rune(msg)[:inner-25]) + "…"
		}
		b.WriteString("  " + marker + dimS.Render(shortSHA(commits[i].SHA)+" ") + style.Render(msg) + "\n")
	}

	b.WriteString("\n  " + hintS.Render("[←/→ or h/l] scrub history · [esc] back") + "\n")
	return b.String()
}

func shortSHA(s string) string {
	if len(s) > 7 {
		return s[:7]
	}
	if s == "" {
		return "-------"
	}
	return s
}

func firstLine(s string) string {
	if i := strings.IndexAny(s, "\r\n"); i >= 0 {
		return s[:i]
	}
	return s
}
