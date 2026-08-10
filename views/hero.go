package views

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// ─────────────────────────────────────────────────────────────────────────
// Hero — the header
//
// The braille portrait sits unboxed in the upper-left, the identity and
// navigation to its right. No borders, no panels: the two columns are joined
// with lipgloss and separated by generous padding, so the space itself does
// the work a box used to do badly.
// ─────────────────────────────────────────────────────────────────────────

// stackWidth is the point below which the two columns stop fitting side by
// side and stack vertically instead.
const stackWidth = 75

// portraitCols is how much of each portrait line is drawn. The art is 68
// columns wide and already cropped to the face, so it is drawn in full.
const portraitCols = 34

// HeroState is what the header needs to draw itself.
type HeroState struct {
	Nav       []string
	NavCursor int
	NavHover  int // -1 when the pointer isn't over an item

	// PortraitRows limits how much of the portrait is drawn, for the boot
	// reveal. Use PortraitRows() for the whole thing.
	PortraitRows int

	// MaxRows caps the header's height so the content pane always has room.
	// The portrait is the hero, but a 24-row portrait on a 30-row terminal
	// leaves nothing to read — it is cropped from the bottom (losing the
	// shoulders, keeping the face) rather than the page losing its content.
	// Zero means no cap.
	MaxRows int

	// SizzleFrame drives the decoder effect on the name, title and nav.
	// Negative means "already resolved".
	SizzleFrame int
}

// navGrid lays the navigation out as a spacious matrix rather than a list.
// Two rows of three reads faster than six stacked lines and leaves the column
// feeling open.
func navGrid(r *lipgloss.Renderer, st HeroState, width int, theme Theme) []string {
	if len(st.Nav) == 0 {
		return nil
	}
	sel := r.NewStyle().Foreground(lipgloss.Color(theme.Primary)).Bold(true)
	hov := r.NewStyle().Foreground(lipgloss.Color(theme.Secondary))
	idle := r.NewStyle().Foreground(lipgloss.Color(theme.DimMid))

	// Widest label, so the columns line up without a table.
	cell := 0
	for _, n := range st.Nav {
		if w := len([]rune(n)) + 3; w > cell {
			cell = w
		}
	}
	perRow := maxInt(width/cell, 1)
	if perRow > 3 {
		perRow = 3 // three across keeps it airy even on a wide terminal
	}

	var rows []string
	var cur strings.Builder
	for i, name := range st.Nav {
		label := name
		if st.SizzleFrame >= 0 {
			label = Sizzle(label, st.SizzleFrame-i*2, 1)
		}

		var cellStr string
		switch {
		case i == st.NavCursor:
			cellStr = sel.Render("▸ " + label)
		case i == st.NavHover:
			cellStr = hov.Render("▸ " + label)
		default:
			cellStr = idle.Render("  " + label)
		}
		cur.WriteString(padToWidth(cellStr, cell))

		if (i+1)%perRow == 0 || i == len(st.Nav)-1 {
			rows = append(rows, strings.TrimRight(cur.String(), " "))
			cur.Reset()
		}
	}
	return rows
}

// identityColumn is the right-hand side: name, title, navigation.
func identityColumn(r *lipgloss.Renderer, st HeroState, width int, theme Theme) []string {
	nameS := r.NewStyle().Foreground(lipgloss.Color(theme.Text)).Bold(true)
	titleS := r.NewStyle().Foreground(lipgloss.Color(theme.DimMid))

	name := strings.ToUpper(TheProfile.Name)
	title := "ML Engineer & Builder"
	if st.SizzleFrame >= 0 {
		name = Sizzle(name, st.SizzleFrame, 1)
		title = Sizzle(title, st.SizzleFrame-6, 1)
	}

	out := []string{
		nameS.Render(name),
		titleS.Render(title),
		"",
		"",
	}
	out = append(out, navGrid(r, st, width, theme)...)
	return out
}

// RenderHero draws the header. Returns the rendered block; the caller decides
// where the rules go.
func RenderHero(r *lipgloss.Renderer, width int, st HeroState, theme Theme) string {
	if width <= 0 {
		return ""
	}
	portraitS := r.NewStyle().Foreground(lipgloss.Color(theme.Primary))

	rows := st.PortraitRows
	if st.MaxRows > 0 && rows > st.MaxRows {
		rows = st.MaxRows
	}

	// ── portrait column ──
	var portrait []string
	for _, line := range PortraitReveal(rows) {
		runes := []rune(line)
		if len(runes) > portraitCols {
			runes = runes[:portraitCols]
		}
		portrait = append(portrait, portraitS.Render(string(runes)))
	}

	// ── narrow: stack, so nothing overlaps or slices out of range ──
	if width < stackWidth {
		var b strings.Builder
		// A narrow terminal can't take 52 columns of art; trim to fit.
		trim := maxInt(width-2, 1)
		for _, line := range PortraitReveal(rows) {
			runes := []rune(line)
			if len(runes) > trim {
				runes = runes[:trim]
			}
			b.WriteString(" " + portraitS.Render(string(runes)) + "\n")
		}
		b.WriteString("\n")
		for _, line := range identityColumn(r, st, maxInt(width-2, 1), theme) {
			b.WriteString(" " + line + "\n")
		}
		return strings.TrimRight(b.String(), "\n")
	}

	// ── wide: two columns, joined, with real breathing room between ──
	left := strings.Join(portrait, "\n")
	rightW := maxInt(width-portraitCols-8, 20)
	right := strings.Join(identityColumn(r, st, rightW, theme), "\n")

	// Push the identity down so it sits against the portrait's eyeline
	// rather than floating at the very top of the art.
	if st.PortraitRows > 6 {
		right = "\n\n\n\n" + right
	}

	return lipgloss.JoinHorizontal(
		lipgloss.Top,
		lipgloss.NewStyle().PaddingLeft(1).Render(left),
		lipgloss.NewStyle().PaddingLeft(6).Render(right),
	)
}

// HeroNavHit maps a pointer position to a navigation index, or -1 when the
// pointer isn't over one. The geometry has to agree with navGrid, so both
// derive the cell size and columns the same way.
func HeroNavHit(width, navCount, x, y int) int {
	if navCount <= 0 || width <= 0 {
		return -1
	}
	cell := 0
	for _, n := range navLabelsFor(navCount) {
		if w := len([]rune(n)) + 3; w > cell {
			cell = w
		}
	}
	if cell == 0 {
		return -1
	}

	var col int
	var originX, originY int
	if width < stackWidth {
		// Stacked: nav sits below the portrait and the identity lines.
		originX, originY = 1, PortraitRows()+3
		col = 1
	} else {
		originX = 1 + portraitCols + 6
		originY = 1 + 4 + 4 // top pad + the identity's leading blank rows
		col = maxInt(minInt((width-originX)/cell, 3), 1)
	}

	dx, dy := x-originX, y-originY
	if dx < 0 || dy < 0 {
		return -1
	}
	row := dy
	slot := dx / cell
	if slot >= col {
		return -1
	}
	idx := row*col + slot
	if idx < 0 || idx >= navCount {
		return -1
	}
	return idx
}

// navLabelsFor is a stand-in so hit-testing can size cells without the caller
// passing the labels through. Widths only depend on the longest label.
func navLabelsFor(n int) []string {
	out := make([]string, 0, n)
	for i, s := range []string{"Projects", "About", "Resume", "Guestbook", "Games", "Quit"} {
		if i >= n {
			break
		}
		out = append(out, s)
	}
	return out
}
