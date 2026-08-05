package views

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// ─────────────────────────────────────────────────────────────────────────
// Shell — the layout engine
//
// Every screen is composed onto a canvas sized exactly to the terminal, so a
// frame is W×H by construction rather than by careful string arithmetic. That
// buys three things the old per-view string building couldn't:
//
//   - it is impossible to overflow or leave the terminal partly blank
//   - the full window is always used, so panels can be laid out as a grid
//   - one place decides borders, titles, padding and truncation
//
// Anything drawn outside its rect is clipped by the canvas, so a panel can
// never bleed into its neighbour no matter how long the content is.
// ─────────────────────────────────────────────────────────────────────────

// Rect is a region of the terminal, in cells.
type Rect struct{ X, Y, W, H int }

// Inner returns the drawable area inside a panel's border.
func (r Rect) Inner() Rect {
	in := Rect{X: r.X + 2, Y: r.Y + 1, W: r.W - 4, H: r.H - 2}
	if in.W < 0 {
		in.W = 0
	}
	if in.H < 0 {
		in.H = 0
	}
	return in
}

// Frame composes one full-terminal picture.
type Frame struct {
	c     *canvas
	r     *lipgloss.Renderer
	theme Theme
	W, H  int
}

// NewFrame allocates a frame the exact size of the terminal.
func NewFrame(r *lipgloss.Renderer, w, h int, theme Theme) *Frame {
	if w < 0 {
		w = 0
	}
	if h < 0 {
		h = 0
	}
	return &Frame{c: newCanvas(w, h), r: r, theme: theme, W: w, H: h}
}

// Render emits the composed frame.
func (f *Frame) Render() string { return f.c.render(f.r) }

// ── primitives ───────────────────────────────────────────────────────────

// Text draws a single line, clipped to maxW columns.
func (f *Frame) Text(x, y int, s string, col lipgloss.Color) {
	f.text(x, y, s, col, false, f.W-x)
}

// TextBold draws an emphasised line.
func (f *Frame) TextBold(x, y int, s string, col lipgloss.Color) {
	f.text(x, y, s, col, true, f.W-x)
}

// TextIn draws a line clipped to a rect's width, so a long value can never
// escape its panel.
func (f *Frame) TextIn(rect Rect, dx, dy int, s string, col lipgloss.Color) {
	f.text(rect.X+dx, rect.Y+dy, s, col, false, rect.W-dx)
}

// TextBoldIn is TextIn with emphasis.
func (f *Frame) TextBoldIn(rect Rect, dx, dy int, s string, col lipgloss.Color) {
	f.text(rect.X+dx, rect.Y+dy, s, col, true, rect.W-dx)
}

func (f *Frame) text(x, y int, s string, col lipgloss.Color, bold bool, maxW int) {
	if maxW <= 0 || y < 0 || y >= f.H {
		return
	}
	i := 0
	for _, ch := range s {
		if i >= maxW {
			// Mark the cut so a truncated value reads as truncated.
			if maxW > 0 {
				f.setCell(x+maxW-1, y, '…', col, bold)
			}
			return
		}
		f.setCell(x+i, y, ch, col, bold)
		i++
	}
}

func (f *Frame) setCell(x, y int, ch rune, col lipgloss.Color, bold bool) {
	if bold {
		f.c.setBold(x, y, ch, col)
	} else {
		f.c.set(x, y, ch, col)
	}
}

// Fill paints every cell of a rect with one rune — used for selection bars.
func (f *Frame) Fill(rect Rect, ch rune, col lipgloss.Color) {
	for y := 0; y < rect.H; y++ {
		for x := 0; x < rect.W; x++ {
			f.c.set(rect.X+x, rect.Y+y, ch, col)
		}
	}
}

// HRule draws a horizontal divider.
func (f *Frame) HRule(x, y, w int, col lipgloss.Color) {
	for i := 0; i < w; i++ {
		f.c.set(x+i, y, '─', col)
	}
}

// ── panels ───────────────────────────────────────────────────────────────

// Panel draws a titled box and returns the drawable interior.
//
// The title sits in the top border. Accent picks out the title so a focused
// panel can be distinguished from an idle one without moving anything.
func (f *Frame) Panel(rect Rect, title string, border, accent lipgloss.Color) Rect {
	if rect.W < 4 || rect.H < 2 {
		return rect.Inner()
	}
	x, y, w, h := rect.X, rect.Y, rect.W, rect.H

	// Corners and edges.
	f.c.set(x, y, '╭', border)
	f.c.set(x+w-1, y, '╮', border)
	f.c.set(x, y+h-1, '╰', border)
	f.c.set(x+w-1, y+h-1, '╯', border)
	for i := 1; i < w-1; i++ {
		f.c.set(x+i, y, '─', border)
		f.c.set(x+i, y+h-1, '─', border)
	}
	for j := 1; j < h-1; j++ {
		f.c.set(x, y+j, '│', border)
		f.c.set(x+w-1, y+j, '│', border)
	}

	// Title inset into the top edge.
	if title != "" && w > 8 {
		label := " " + title + " "
		if len([]rune(label)) > w-4 {
			label = " " + string([]rune(title)[:maxInt(w-8, 1)]) + "… "
		}
		f.text(x+2, y, label, accent, true, w-4)
	}
	return rect.Inner()
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// ── layout helpers ───────────────────────────────────────────────────────

// SplitV cuts a rect into a left column of exactly leftW and the remainder,
// with an optional gap between them.
func SplitV(r Rect, leftW, gap int) (left, right Rect) {
	if leftW > r.W {
		leftW = r.W
	}
	left = Rect{X: r.X, Y: r.Y, W: leftW, H: r.H}
	rx := r.X + leftW + gap
	rw := r.W - leftW - gap
	if rw < 0 {
		rw = 0
	}
	right = Rect{X: rx, Y: r.Y, W: rw, H: r.H}
	return
}

// SplitH cuts a rect into a top band of exactly topH and the remainder.
func SplitH(r Rect, topH, gap int) (top, bottom Rect) {
	if topH > r.H {
		topH = r.H
	}
	top = Rect{X: r.X, Y: r.Y, W: r.W, H: topH}
	by := r.Y + topH + gap
	bh := r.H - topH - gap
	if bh < 0 {
		bh = 0
	}
	bottom = Rect{X: r.X, Y: by, W: r.W, H: bh}
	return
}

// Columns divides a rect into n equal columns separated by gap, distributing
// the remainder so the columns together span the full width with no gap left
// over at the right edge.
func Columns(r Rect, n, gap int) []Rect {
	if n <= 0 {
		return nil
	}
	total := r.W - gap*(n-1)
	if total < n {
		total = n
	}
	base, extra := total/n, total%n

	out := make([]Rect, n)
	x := r.X
	for i := 0; i < n; i++ {
		w := base
		if i < extra {
			w++ // spread the remainder across the leftmost columns
		}
		out[i] = Rect{X: x, Y: r.Y, W: w, H: r.H}
		x += w + gap
	}
	return out
}

// ── content helpers ──────────────────────────────────────────────────────

// WrapInto lays text into a rect, returning the number of rows used.
func (f *Frame) WrapInto(rect Rect, s string, col lipgloss.Color) int {
	if rect.W <= 0 || rect.H <= 0 {
		return 0
	}
	lines := strings.Split(wrapWidth(s, rect.W), "\n")
	n := minInt(len(lines), rect.H)
	for i := 0; i < n; i++ {
		f.text(rect.X, rect.Y+i, lines[i], col, false, rect.W)
	}
	return n
}

// KeyValue draws an aligned label/value row, the pattern used throughout the
// rail and info panels.
func (f *Frame) KeyValue(rect Rect, dy int, key, val string, keyW int,
	keyCol, valCol lipgloss.Color) {
	if dy < 0 || dy >= rect.H {
		return
	}
	f.text(rect.X, rect.Y+dy, key, keyCol, false, minInt(keyW, rect.W))
	if vx := keyW + 1; vx < rect.W {
		f.text(rect.X+vx, rect.Y+dy, val, valCol, false, rect.W-vx)
	}
}

// Sparkline renders values as a bar strip. Values are normalised to their own
// maximum, so the shape is readable whatever the absolute numbers are.
func Sparkline(vals []int, width int) string {
	const bars = "▁▂▃▄▅▆▇█"
	runes := []rune(bars)
	if width <= 0 || len(vals) == 0 {
		return ""
	}
	max := 1
	for _, v := range vals {
		if v > max {
			max = v
		}
	}
	var b strings.Builder
	for i := 0; i < width; i++ {
		// Sample the series across the available width.
		v := vals[i*len(vals)/width]
		idx := v * (len(runes) - 1) / max
		if idx < 0 {
			idx = 0
		}
		if idx >= len(runes) {
			idx = len(runes) - 1
		}
		b.WriteRune(runes[idx])
	}
	return b.String()
}

// StatusDot returns the glyph and colour for a project status.
func StatusDot(status string, pulse bool, theme Theme) (rune, lipgloss.Color) {
	switch status {
	case "Live":
		if pulse {
			return '●', lipgloss.Color(theme.Success)
		}
		return '○', lipgloss.Color(theme.Dim)
	case "WIP":
		return '◐', lipgloss.Color(theme.Warning)
	case "Research":
		return '◇', lipgloss.Color(theme.Purple)
	default:
		return '·', lipgloss.Color(theme.Dim)
	}
}
