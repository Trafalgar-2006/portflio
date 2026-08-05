package views

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// ─────────────────────────────────────────────────────────────────────────
// Cockpit — the home screen
//
// A persistent left rail (identity, navigation, live vitals) beside a grid of
// panels that fills whatever the terminal gives us. Nothing is hidden behind
// navigation: the work, what he's doing now, the live signal and an ambient
// effect are all on screen at once.
//
// Composed onto a Frame, so it is exactly W×H at any size and degrades by
// dropping panels rather than overflowing.
// ─────────────────────────────────────────────────────────────────────────

// railWidth is the fixed nav column. Below cockpitMinWidth the rail is dropped
// and the layout falls back to a single column.
const (
	railWidth        = 24
	cockpitMinWidth  = 64
	cockpitMinHeight = 18
)

// CockpitState is everything the home screen renders. Passing one struct keeps
// the signature stable as panels come and go.
type CockpitState struct {
	NavItems   []string
	NavCursor  int
	Session    string
	Connected  string
	Ping       int
	Online     int64
	Uptime     string
	Build      string
	LastCommit string
	Pulse      bool
	Tick       int

	// Ambient is a pre-rendered effect frame; empty means no ambient panel.
	Ambient     []string
	AmbientName string

	// Reveal drives the content-first entrance: 0 = nothing, rising to 100.
	Reveal int
}

// RenderCockpit draws the home screen.
func RenderCockpit(r *lipgloss.Renderer, w, h int, st CockpitState, theme Theme) string {
	f := NewFrame(r, w, h, theme)
	if w <= 0 || h <= 0 {
		return f.Render()
	}

	border := lipgloss.Color(theme.BoxBorder)
	accent := lipgloss.Color(theme.Primary)
	dim := lipgloss.Color(theme.Dim)
	mid := lipgloss.Color(theme.DimMid)
	text := lipgloss.Color(theme.Text)

	// Leave the last row for the status bar.
	body := Rect{X: 0, Y: 0, W: w, H: maxInt(h-1, 1)}

	var main Rect
	if w >= cockpitMinWidth && h >= cockpitMinHeight {
		var rail Rect
		rail, main = SplitV(body, railWidth, 0)
		drawRail(f, rail, st, theme)
	} else {
		main = body // too small for a rail; content gets everything
	}

	drawMain(f, main, st, theme)
	drawStatusBar(f, Rect{X: 0, Y: h - 1, W: w, H: 1}, st, theme)

	_ = border
	_ = accent
	_ = dim
	_ = mid
	_ = text
	return f.Render()
}

// drawRail paints identity, navigation and live vitals down the left edge.
func drawRail(f *Frame, rect Rect, st CockpitState, theme Theme) {
	border := lipgloss.Color(theme.BoxBorder)
	accent := lipgloss.Color(theme.Primary)
	dim := lipgloss.Color(theme.Dim)
	mid := lipgloss.Color(theme.DimMid)
	text := lipgloss.Color(theme.Text)
	good := lipgloss.Color(theme.Success)

	in := f.Panel(rect, "", border, accent)
	y := 0

	// ── identity ──
	name := strings.ToUpper(TheProfile.Name)
	parts := strings.Fields(name)
	for _, p := range parts {
		if y >= in.H {
			break
		}
		f.TextBoldIn(in, 0, y, p, accent)
		y++
	}
	if y < in.H {
		f.TextIn(in, 0, y, TheProfile.Location, dim)
		y += 2
	}

	// ── navigation ──
	if y < in.H {
		f.TextIn(in, 0, y, "NAVIGATE", mid)
		y++
	}
	for i, item := range st.NavItems {
		if y >= in.H-6 {
			break
		}
		if i == st.NavCursor {
			// Full-width inverse bar: the selection must be unmistakable.
			f.Fill(Rect{X: in.X, Y: in.Y + y, W: in.W, H: 1}, ' ', accent)
			f.c.setBold(in.X, in.Y+y, '▸', accent)
			for j, ch := range item {
				if 2+j >= in.W {
					break
				}
				f.c.setBold(in.X+2+j, in.Y+y, ch, accent)
			}
		} else {
			f.TextIn(in, 2, y, item, mid)
		}
		y++
	}

	// ── push activity, filling the gap above the vitals ──
	// Real data: the commit history the timeline view already fetches.
	if act := CommitActivity(in.W); len(act) > 0 && y+3 < in.H-6 {
		y++
		f.TextIn(in, 0, y, "PUSH ACTIVITY", mid)
		y++
		f.TextIn(in, 0, y, Sparkline(act, in.W), lipgloss.Color(theme.Success))
		y++
	}

	// ── vitals, pinned to the bottom ──
	vy := in.H - 5
	if vy > y+1 {
		f.HRule(in.X, in.Y+vy-1, in.W, border)
		rows := [][2]string{
			{"online", fmt.Sprintf("%d", st.Online)},
			{"latency", fmt.Sprintf("%dms", st.Ping)},
			{"session", st.Session},
			{"up", st.Connected},
		}
		for i, kv := range rows {
			if vy+i >= in.H {
				break
			}
			col := text
			if i == 0 && st.Online > 0 {
				col = good
			}
			f.KeyValue(in, vy+i, kv[0], kv[1], 8, dim, col)
		}
	}
}

// drawMain lays the panel grid: a top band (identity + now), the work list,
// and an ambient strip.
func drawMain(f *Frame, rect Rect, st CockpitState, theme Theme) {
	if rect.W < 8 || rect.H < 6 {
		return
	}
	border := lipgloss.Color(theme.BoxBorder)
	accent := lipgloss.Color(theme.Primary)

	// Ambient strip only when there's genuine room to spare.
	ambientH := 0
	if len(st.Ambient) > 0 && rect.H >= 26 {
		ambientH = minInt(7, rect.H/4)
	}

	topH := minInt(9, maxInt(rect.H/3, 5))
	if rect.H-topH-ambientH < 6 {
		topH = maxInt(rect.H-ambientH-6, 3)
	}

	top, rest := SplitH(rect, topH, 0)
	work, ambient := rest, Rect{}
	if ambientH > 0 {
		work, ambient = SplitH(rest, rest.H-ambientH, 0)
	}

	// ── top band: headline beside /now ──
	if top.W >= 56 {
		cols := Columns(top, 2, 0)
		drawHeadline(f, cols[0], st, theme)
		drawNow(f, cols[1], theme)
	} else {
		drawHeadline(f, top, st, theme)
	}

	drawWork(f, work, st, theme)

	if ambientH > 0 {
		title := "AMBIENT"
		if st.AmbientName != "" {
			title = "AMBIENT · " + st.AmbientName
		}
		in := f.Panel(ambient, title, border, accent)
		drawAmbient(f, in, st, theme)
	}
}

// drawHeadline is the identity panel: what he does, in his words.
func drawHeadline(f *Frame, rect Rect, st CockpitState, theme Theme) {
	border := lipgloss.Color(theme.BoxBorder)
	accent := lipgloss.Color(theme.Primary)
	text := lipgloss.Color(theme.Text)
	dim := lipgloss.Color(theme.Dim)
	pink := lipgloss.Color(theme.Secondary)

	mid := lipgloss.Color(theme.DimMid)

	in := f.Panel(rect, "WHO", border, accent)
	if in.H <= 0 {
		return
	}
	y := 0
	f.TextBoldIn(in, 0, y, TheProfile.Tagline, pink)
	y += 2

	// Typewriter entrance: reveal proportionally to st.Reveal.
	tag := TaglineText
	if st.Reveal < 100 {
		n := len([]rune(tag)) * st.Reveal / 100
		tag = string([]rune(tag)[:n])
	}
	y += f.WrapInto(Rect{X: in.X, Y: in.Y + y, W: in.W, H: maxInt(in.H-y-1, 0)}, tag, text)
	y++

	// Fill the remaining rows with the current roles — the credential a
	// visitor is actually scanning for, rather than empty space.
	for _, role := range AllExperience {
		if y >= in.H-1 {
			break
		}
		f.text(in.X, in.Y+y, role.Title, mid, false, in.W)
		if org := role.Org + " · " + role.Period; len([]rune(role.Title))+2 < in.W {
			f.text(in.X+len([]rune(role.Title))+2, in.Y+y, org, dim, false,
				in.W-len([]rune(role.Title))-2)
		}
		y++
	}

	if in.H > 0 && st.LastCommit != "" {
		f.TextIn(in, 0, in.H-1, "last push "+st.LastCommit, dim)
	}
}

// drawNow mirrors the /now page in miniature, so the freshest thing about him
// is on the landing screen rather than a tab away.
func drawNow(f *Frame, rect Rect, theme Theme) {
	border := lipgloss.Color(theme.BoxBorder)
	accent := lipgloss.Color(theme.Primary)
	dim := lipgloss.Color(theme.Dim)
	text := lipgloss.Color(theme.Text)
	good := lipgloss.Color(theme.Success)

	in := f.Panel(rect, "NOW", border, accent)
	y := 0
	sections := []struct {
		label string
		items []string
	}{
		{"building", TheNow.Building},
		{"learning", TheNow.Learning},
	}
	for _, s := range sections {
		for i, item := range s.items {
			if y >= in.H {
				return
			}
			label := ""
			if i == 0 {
				label = s.label
			}
			f.text(in.X, in.Y+y, fmt.Sprintf("%-9s", label), dim, false, minInt(9, in.W))
			if 10 < in.W {
				f.c.set(in.X+9, in.Y+y, '▸', good)
				f.text(in.X+11, in.Y+y, item, text, false, in.W-11)
			}
			y++
		}
	}
}

// drawWork is the project list — the reason anyone is here.
func drawWork(f *Frame, rect Rect, st CockpitState, theme Theme) {
	border := lipgloss.Color(theme.BoxBorder)
	accent := lipgloss.Color(theme.Primary)
	dim := lipgloss.Color(theme.Dim)
	mid := lipgloss.Color(theme.DimMid)
	text := lipgloss.Color(theme.Text)

	projects := Projects()
	title := fmt.Sprintf("WORK · %d", len(projects))
	in := f.Panel(rect, title, border, accent)
	if in.H <= 0 || in.W <= 0 {
		return
	}

	// Stagger the entrance: one row per few percent of reveal.
	visible := len(projects)
	if st.Reveal < 100 {
		visible = minInt(len(projects), st.Reveal*len(projects)/80+1)
	}

	// Adaptive density: spend spare vertical space on more detail per project
	// rather than leaving the panel half empty. One row is a bare list; two
	// adds the description; three adds the repo link.
	avail := in.H - 1 // keep the last row for the hint
	rowsPer := 1
	if n := len(projects); n > 0 {
		rowsPer = clampInt(avail/n, 1, 3)
	}

	y := 0
	for i, p := range projects {
		if y >= avail || i >= visible {
			break
		}
		dot, dotCol := StatusDot(p.Status, st.Pulse, theme)
		f.c.set(in.X, in.Y+y, dot, dotCol)

		// Reserve the right edge for the highlight so tags can't collide.
		hl := ""
		if p.Highlight != "" {
			hl = "⚡" + p.Highlight
		}
		hlW := len([]rune(hl))
		if hlW > 0 {
			hlW++ // a column of breathing room
		}
		contentW := maxInt(in.W-2-hlW, 8)

		if rowsPer == 1 {
			// Single row: title, then tags in whatever is left.
			titleW := minInt(34, maxInt(contentW/2, 12))
			f.text(in.X+2, in.Y+y, p.Title, text, false, titleW)
			if tx := 2 + titleW + 1; tx-2 < contentW {
				f.text(in.X+tx, in.Y+y, strings.Join(p.Tags, " "), mid, false, contentW-(tx-2))
			}
		} else {
			f.text(in.X+2, in.Y+y, p.Title, text, false, contentW)
			if y+1 < avail {
				f.text(in.X+4, in.Y+y+1, firstSentence(p.Description), dim, false, contentW-2)
			}
			if rowsPer >= 3 && y+2 < avail {
				line := strings.Join(p.Tags, " ")
				if p.GitHubURL != "" {
					line += "   → " + p.GitHubURL
				}
				f.text(in.X+4, in.Y+y+2, line, mid, false, contentW-2)
			}
		}

		if hl != "" {
			f.text(in.X+in.W-hlW, in.Y+y, hl, lipgloss.Color(theme.Accent), false, hlW)
		}
		y += rowsPer
	}

	// Footer hint on the panel's last row.
	if in.H > 0 {
		f.text(in.X, in.Y+in.H-1, "⏎ open · / search anything · ? for every key", dim, false, in.W)
	}
}

// firstSentence trims a description to its opening sentence, so a list row
// stays a list row.
func firstSentence(s string) string {
	if i := strings.Index(s, ". "); i > 0 {
		return s[:i+1]
	}
	return s
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// CockpitAmbientSize reports the interior size of the ambient panel for a
// given terminal, or (0,0) when the layout has no room for one. The model uses
// it to size its effect so the frame fills the panel exactly.
func CockpitAmbientSize(w, h int) (int, int) {
	if w <= 0 || h <= 0 {
		return 0, 0
	}
	body := Rect{X: 0, Y: 0, W: w, H: maxInt(h-1, 1)}
	main := body
	if w >= cockpitMinWidth && h >= cockpitMinHeight {
		_, main = SplitV(body, railWidth, 0)
	}
	if main.W < 8 || main.H < 6 || main.H < 26 {
		return 0, 0
	}
	ambientH := minInt(7, main.H/4)
	ambient := Rect{X: main.X, Y: 0, W: main.W, H: ambientH}
	in := ambient.Inner()
	return maxInt(in.W, 0), maxInt(in.H, 0)
}

// drawAmbient paints the live effect frame, dimmed so it reads as atmosphere
// rather than competing with the content above it.
func drawAmbient(f *Frame, in Rect, st CockpitState, theme Theme) {
	col := lipgloss.Color(theme.Dim)
	for i, line := range st.Ambient {
		if i >= in.H {
			break
		}
		f.text(in.X, in.Y+i, line, col, false, in.W)
	}
}

// drawStatusBar is the single always-visible affordance line.
func drawStatusBar(f *Frame, rect Rect, st CockpitState, theme Theme) {
	bg := lipgloss.Color(theme.FooterBg)
	dim := lipgloss.Color(theme.FooterText)
	accent := lipgloss.Color(theme.Primary)

	f.Fill(rect, ' ', bg)

	// Left: the keys that matter, with the discoverable one first.
	left := " ? help   / search   ⏎ open   t theme   s fx   q quit"
	f.text(rect.X+1, rect.Y, left, dim, false, rect.W-2)

	// Right: theme name, so switching is visibly confirmed.
	right := theme.Name
	if x := rect.W - len([]rune(right)) - 2; x > len([]rune(left))+2 {
		f.text(rect.X+x, rect.Y, right, accent, false, rect.W-x)
	}
}
