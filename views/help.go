package views

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// ─────────────────────────────────────────────────────────────────────────
// Help overlay
//
// Everything this portfolio can do, on one screen, reachable from anywhere
// with `?`. Without it the effects engine, the games, the guestbook and the
// theme switcher are all effectively invisible — a visitor has no way to learn
// that pressing `s` opens a playground of eight animations.
//
// The list below is the single source of truth for what's advertised, so a
// binding added to the model and not added here shows up as a test failure.
// ─────────────────────────────────────────────────────────────────────────

// Binding is one advertised key.
type Binding struct {
	Keys string
	What string
}

// HelpGroup is a titled cluster of bindings.
type HelpGroup struct {
	Title    string
	Bindings []Binding
}

// HelpGroups is the full keymap, ordered by how likely someone is to want it.
func HelpGroups() []HelpGroup {
	return []HelpGroup{
		{"Getting around", []Binding{
			{"↑ ↓ / j k", "move the selection"},
			{"⏎", "open what's selected"},
			{"esc", "back"},
			{"/ or ⌃K", "search anything — projects, pages, games, effects"},
			{"? ", "this screen"},
			{"q q", "quit (twice to confirm)"},
		}},
		{"Reading", []Binding{
			{"PgUp PgDn", "page up and down"},
			{"⌃U ⌃D", "half a page"},
			{"g g / G", "jump to top / bottom"},
			{"3j 5k", "vim counts work too"},
			{"c", "copy-friendly contacts (plain text, no styling)"},
		}},
		{"Making it yours", []Binding{
			{"t", "cycle theme — dracula, matrix, amber, nord, cyberpunk"},
			{"w", "wave distortion"},
		}},
		{"Things to play with", []Binding{
			{"s", "effects playground — 8 animations, mouse-reactive"},
			{"n p", "next / previous effect (inside the playground)"},
			{"l", "lock the current effect"},
			{"click", "interact — pour sand, stamp a glider, ignite fire"},
			{"i", "sign the guestbook"},
		}},
		{"From your own shell", []Binding{
			{"ssh … projects", "skip the intro, open a page directly"},
			{"ssh … snake", "or tetris, fx, guestbook, resume, now"},
		}},
	}
}

// RenderHelp draws the overlay full-screen.
func RenderHelp(r *lipgloss.Renderer, w, h int, theme Theme) string {
	f := NewFrame(r, w, h, theme)
	if w <= 0 || h <= 0 {
		return f.Render()
	}

	border := lipgloss.Color(theme.BoxBorder)
	accent := lipgloss.Color(theme.Primary)
	keyCol := lipgloss.Color(theme.Accent)
	text := lipgloss.Color(theme.Text)
	dim := lipgloss.Color(theme.Dim)
	mid := lipgloss.Color(theme.DimMid)

	in := f.Panel(Rect{X: 0, Y: 0, W: w, H: h}, "EVERY KEY", border, accent)
	if in.W <= 0 || in.H <= 0 {
		return f.Render()
	}

	groups := HelpGroups()

	// Two columns when there's room; the keymap is long enough that one column
	// scrolls off a normal terminal.
	cols := 1
	if in.W >= 78 {
		cols = 2
	}
	colRects := Columns(in, cols, 4)

	// Distribute groups across columns by weight, so neither column runs long.
	perCol := make([][]HelpGroup, cols)
	rows := make([]int, cols)
	for _, g := range groups {
		// Pick the shortest column.
		best := 0
		for i := 1; i < cols; i++ {
			if rows[i] < rows[best] {
				best = i
			}
		}
		perCol[best] = append(perCol[best], g)
		rows[best] += len(g.Bindings) + 2
	}

	// Widest key string, so descriptions align down the page.
	keyW := 0
	for _, g := range groups {
		for _, b := range g.Bindings {
			if n := len([]rune(strings.TrimSpace(b.Keys))); n > keyW {
				keyW = n
			}
		}
	}
	keyW += 2

	for ci, rect := range colRects {
		y := 0
		for _, g := range perCol[ci] {
			if y >= rect.H {
				break
			}
			f.TextBoldIn(rect, 0, y, strings.ToUpper(g.Title), mid)
			y++
			for _, b := range g.Bindings {
				if y >= rect.H {
					break
				}
				f.TextIn(rect, 0, y, strings.TrimSpace(b.Keys), keyCol)
				if keyW < rect.W {
					f.TextIn(rect, keyW, y, b.What, text)
				}
				y++
			}
			y++
		}
	}

	// Footer.
	if in.H > 0 {
		f.TextIn(in, 0, in.H-1, "any key to close", dim)
	}
	return f.Render()
}
