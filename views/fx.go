package views

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// ─────────────────────────────────────────────────────────────────────────
// FX engine
//
// Every animation draws into a fixed-size `canvas` of coloured cells rather
// than assembling strings directly. That gives three things the previous
// ad-hoc effects lacked:
//
//   - bounds safety in exactly one place (canvas.set clips silently), so an
//     effect can never index out of range no matter how odd the terminal size
//   - run-length grouped ANSI output, so one row of 200 same-coloured cells
//     costs one escape sequence instead of 200 — this matters over SSH
//   - a uniform Effect interface, so the screensaver can cycle anything
// ─────────────────────────────────────────────────────────────────────────

// Effect is a self-contained full-screen animation.
type Effect interface {
	// Name is the label shown in the screensaver chrome.
	Name() string
	// Resize re-seeds state for a new terminal size.
	Resize(w, h int)
	// Step advances exactly one frame.
	Step()
	// Render draws the current frame.
	Render(r *lipgloss.Renderer, theme Theme) string
	// Interact drops something at a cell, for mouse-reactive effects.
	// Effects that ignore input implement this as a no-op.
	Interact(x, y int)
}

// TextEffect animates a block of text into place. Unlike Effect it terminates:
// Step reports false once the text has fully settled.
type TextEffect interface {
	Name() string
	// Start seeds the animation for these lines within a w×h area.
	Start(lines []string, w, h int)
	// Step advances a frame and reports whether the animation is still running.
	Step() bool
	// Done reports whether the text has fully settled.
	Done() bool
	Render(r *lipgloss.Renderer, theme Theme) string
}

// ─────────────────────────── canvas ─────────────────────────────────────

// canvas is a grid of runes with per-cell colours, rendered with run-length
// grouped ANSI so wide flat regions cost one escape sequence rather than one
// per cell.
type canvas struct {
	w, h  int
	chars []rune
	cols  []lipgloss.Color
	bold  []bool
}

func newCanvas(w, h int) *canvas {
	if w < 0 {
		w = 0
	}
	if h < 0 {
		h = 0
	}
	c := &canvas{w: w, h: h,
		chars: make([]rune, w*h),
		cols:  make([]lipgloss.Color, w*h),
		bold:  make([]bool, w*h),
	}
	c.clear()
	return c
}

func (c *canvas) clear() {
	for i := range c.chars {
		c.chars[i] = ' '
		c.cols[i] = ""
		c.bold[i] = false
	}
}

// set writes a cell, silently ignoring out-of-bounds coordinates. Effects rely
// on this: it's what makes them safe at any terminal size.
func (c *canvas) set(x, y int, ch rune, col lipgloss.Color) {
	if x < 0 || y < 0 || x >= c.w || y >= c.h {
		return
	}
	i := y*c.w + x
	c.chars[i] = ch
	c.cols[i] = col
}

func (c *canvas) setBold(x, y int, ch rune, col lipgloss.Color) {
	if x < 0 || y < 0 || x >= c.w || y >= c.h {
		return
	}
	c.set(x, y, ch, col)
	c.bold[y*c.w+x] = true
}

// text writes a string starting at (x,y), clipping at the canvas edge.
func (c *canvas) text(x, y int, s string, col lipgloss.Color) {
	for i, ch := range []rune(s) {
		c.set(x+i, y, ch, col)
	}
}

// render emits the canvas, grouping runs of identical style into one span.
func (c *canvas) render(r *lipgloss.Renderer) string {
	if c.w == 0 || c.h == 0 {
		return ""
	}
	var b strings.Builder
	b.Grow(c.w * c.h * 2)

	var run strings.Builder
	for y := 0; y < c.h; y++ {
		runCol, runBold := lipgloss.Color(""), false
		run.Reset()

		flush := func() {
			if run.Len() == 0 {
				return
			}
			if runCol == "" {
				b.WriteString(run.String()) // unstyled — no escape sequence at all
			} else {
				st := r.NewStyle().Foreground(runCol)
				if runBold {
					st = st.Bold(true)
				}
				b.WriteString(st.Render(run.String()))
			}
			run.Reset()
		}

		for x := 0; x < c.w; x++ {
			i := y*c.w + x
			col, bold := c.cols[i], c.bold[i]
			if col != runCol || bold != runBold {
				flush()
				runCol, runBold = col, bold
			}
			run.WriteRune(c.chars[i])
		}
		flush()
		if y < c.h-1 {
			b.WriteByte('\n')
		}
	}
	return b.String()
}

// ─────────────────────────── shared helpers ──────────────────────────────

// rampChars is the classic luminance ramp, darkest to brightest. Effects map a
// 0..1 intensity onto it.
var rampChars = []rune{' ', '.', ':', '-', '=', '+', '*', '#', '%', '@'}

// rampAt maps intensity 0..1 onto rampChars.
func rampAt(v float64) rune {
	if v <= 0 {
		return ' '
	}
	if v >= 1 {
		return rampChars[len(rampChars)-1]
	}
	i := int(v * float64(len(rampChars)))
	if i >= len(rampChars) {
		i = len(rampChars) - 1
	}
	return rampChars[i]
}

// heatRamp maps 0..1 onto a theme's warm gradient, coolest to hottest.
func heatRamp(v float64, theme Theme) lipgloss.Color {
	switch {
	case v <= 0.00:
		return ""
	case v < 0.18:
		return lipgloss.Color(theme.VeryDim)
	case v < 0.34:
		return lipgloss.Color(theme.Dim)
	case v < 0.50:
		return lipgloss.Color(theme.Purple)
	case v < 0.66:
		return lipgloss.Color(theme.Secondary)
	case v < 0.80:
		return lipgloss.Color(theme.Warning)
	case v < 0.92:
		return lipgloss.Color(theme.Accent)
	default:
		return lipgloss.Color(theme.Text)
	}
}

// depthRamp maps 0..1 (near..far) onto a theme's cool gradient.
func depthRamp(v float64, theme Theme) lipgloss.Color {
	switch {
	case v < 0.20:
		return lipgloss.Color(theme.Text)
	case v < 0.40:
		return lipgloss.Color(theme.Primary)
	case v < 0.60:
		return lipgloss.Color(theme.Secondary)
	case v < 0.78:
		return lipgloss.Color(theme.DimMid)
	case v < 0.92:
		return lipgloss.Color(theme.Dim)
	default:
		return lipgloss.Color(theme.VeryDim)
	}
}

func clampF(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
