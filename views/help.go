package views

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// ─────────────────────────────────────────────────────────────────────────
// Help
//
// Everything the portfolio can do, on one flat screen, reachable with `?`.
// Same rules as everywhere else: one column, no boxes, generous space.
//
// This list is the single source of truth for what's advertised — a binding
// added to the model but not added here fails a test.
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
			{"↑ ↓", "move through the list"},
			{"⏎", "open what's selected"},
			{"esc", "back"},
			{"/", "search anything — pages, projects, games, effects"},
			{"?", "this screen"},
			{"q", "quit"},
		}},
		{"Reading", []Binding{
			{"j k", "line by line"},
			{"PgUp PgDn", "a page at a time"},
			{"g G", "top, bottom"},
			{"c", "copy-friendly contacts"},
		}},
		{"Making it yours", []Binding{
			{"t", "cycle theme — dracula, matrix, amber, nord, cyberpunk"},
			{"w", "wave distortion"},
		}},
		{"Things to play with", []Binding{
			{"s", "effects playground — 8 animations"},
			{"n p", "next, previous effect"},
			{"click", "pour sand, stamp a glider, ignite fire"},
			{"i", "sign the guestbook"},
		}},
		{"From your own shell", []Binding{
			{"ssh … projects", "open a page directly, skipping the intro"},
			{"ssh … snake", "also: tetris, fx, guestbook, resume, now"},
		}},
	}
}

// BlockHelp renders the keymap as flat rows, matching every other block.
func BlockHelp(r *lipgloss.Renderer, w int, theme Theme) []string {
	h2 := r.NewStyle().Foreground(lipgloss.Color(theme.Primary)).Bold(true)
	groupS := r.NewStyle().Foreground(lipgloss.Color(theme.Dim))
	keyS := r.NewStyle().Foreground(lipgloss.Color(theme.Accent))
	whatS := r.NewStyle().Foreground(lipgloss.Color(theme.DimMid))
	hintS := r.NewStyle().Foreground(lipgloss.Color(theme.Dim))

	groups := HelpGroups()

	// Widest key string, so descriptions align down the whole page.
	keyW := 0
	for _, g := range groups {
		for _, b := range g.Bindings {
			if n := len([]rune(b.Keys)); n > keyW {
				keyW = n
			}
		}
	}
	keyW += 4

	out := []string{indent(h2.Render("Keys")), ""}
	for _, g := range groups {
		out = append(out, indent(groupS.Render(strings.ToUpper(g.Title))), "")
		for _, b := range g.Bindings {
			row := padToWidth(keyS.Render(b.Keys), keyW) + whatS.Render(b.What)
			out = append(out, indent("  "+row))
		}
		out = append(out, "")
	}
	out = append(out, indent(hintS.Render("any key to close")))
	return out
}
