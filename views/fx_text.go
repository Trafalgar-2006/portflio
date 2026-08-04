package views

import (
	"math"
	"math/rand"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// glyph is one character of a text effect, tracking where it is now and where
// it belongs once the animation settles.
type glyph struct {
	Ch             rune
	X, Y           float64 // current position
	TX, TY         float64 // target (home) position
	VX, VY         float64 // velocity
	Settled        bool
	Delay          int // frames to wait before this glyph starts moving
	Scramble       rune
	ScrambleFrames int
}

// glyphsFromLines flattens rendered text into positioned glyphs. ANSI is
// stripped first: effects move characters individually, so per-character
// styling can't survive anyway.
func glyphsFromLines(lines []string) []glyph {
	var gs []glyph
	for y, ln := range lines {
		for x, ch := range []rune(stripAnsi(ln)) {
			if ch == ' ' {
				continue
			}
			gs = append(gs, glyph{Ch: ch, TX: float64(x), TY: float64(y)})
		}
	}
	return gs
}

// renderGlyphs paints settled/unsettled glyphs onto a canvas.
func renderGlyphs(r *lipgloss.Renderer, gs []glyph, w, h int, theme Theme, moving lipgloss.Color) string {
	c := newCanvas(w, h)
	for _, g := range gs {
		ch := g.Ch
		if g.ScrambleFrames > 0 && g.Scramble != 0 {
			ch = g.Scramble
		}
		col := moving
		if g.Settled {
			col = lipgloss.Color(theme.Text)
		}
		c.set(int(g.X+0.5), int(g.Y+0.5), ch, col)
	}
	return c.render(r)
}

func allSettled(gs []glyph) bool {
	for i := range gs {
		if !gs[i].Settled {
			return false
		}
	}
	return true
}

// ─────────────────────────── Slot machine ────────────────────────────────

// SlotTextEffect spins each column like a combination lock until the correct
// character clicks into place, left to right.
type SlotTextEffect struct {
	w, h    int
	glyphs  []glyph
	frame   int
	alphabet []rune
}

func NewSlotTextEffect() *SlotTextEffect {
	return &SlotTextEffect{alphabet: []rune("ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789#$%&@*")}
}

func (e *SlotTextEffect) Name() string { return "slot" }

func (e *SlotTextEffect) Start(lines []string, w, h int) {
	e.w, e.h, e.frame = w, h, 0
	e.glyphs = glyphsFromLines(lines)
	for i := range e.glyphs {
		g := &e.glyphs[i]
		g.X, g.Y = g.TX, g.TY // slot machine never moves, it only cycles glyphs
		// Stagger the stop time left-to-right so it reads as a dial settling.
		g.Delay = int(g.TX)*2 + rand.Intn(6) + 8
		g.ScrambleFrames = g.Delay
		g.Scramble = e.alphabet[rand.Intn(len(e.alphabet))]
	}
}

func (e *SlotTextEffect) Step() bool {
	e.frame++
	for i := range e.glyphs {
		g := &e.glyphs[i]
		if g.Settled {
			continue
		}
		if g.ScrambleFrames > 0 {
			g.ScrambleFrames--
			g.Scramble = e.alphabet[rand.Intn(len(e.alphabet))]
			continue
		}
		g.Settled = true
	}
	return !e.Done()
}

func (e *SlotTextEffect) Done() bool { return allSettled(e.glyphs) }

func (e *SlotTextEffect) Render(r *lipgloss.Renderer, theme Theme) string {
	return renderGlyphs(r, e.glyphs, e.w, e.h, theme, lipgloss.Color(theme.Accent))
}

// ─────────────────────────── Swarm ───────────────────────────────────────

// SwarmTextEffect scatters the glyphs, lets them buzz around like bees, then
// snaps them home.
type SwarmTextEffect struct {
	w, h   int
	glyphs []glyph
	frame  int
}

func NewSwarmTextEffect() *SwarmTextEffect { return &SwarmTextEffect{} }

func (e *SwarmTextEffect) Name() string { return "swarm" }

func (e *SwarmTextEffect) Start(lines []string, w, h int) {
	e.w, e.h, e.frame = w, h, 0
	e.glyphs = glyphsFromLines(lines)
	for i := range e.glyphs {
		g := &e.glyphs[i]
		g.X = rand.Float64() * float64(w)
		g.Y = rand.Float64() * float64(h)
		g.Delay = rand.Intn(14)
	}
}

func (e *SwarmTextEffect) Step() bool {
	e.frame++
	for i := range e.glyphs {
		g := &e.glyphs[i]
		if g.Settled {
			continue
		}
		if g.Delay > 0 {
			g.Delay--
			// Buzz in place while waiting.
			g.X += (rand.Float64() - 0.5) * 1.4
			g.Y += (rand.Float64() - 0.5) * 0.8
			continue
		}
		// Steer toward home with a little jitter, then ease in.
		dx, dy := g.TX-g.X, g.TY-g.Y
		g.VX = g.VX*0.72 + dx*0.20 + (rand.Float64()-0.5)*0.30
		g.VY = g.VY*0.72 + dy*0.20 + (rand.Float64()-0.5)*0.18
		g.X += g.VX
		g.Y += g.VY
		if math.Abs(dx) < 0.5 && math.Abs(dy) < 0.5 {
			g.X, g.Y, g.Settled = g.TX, g.TY, true
		}
	}
	// Hard stop so a stray glyph can't keep the animation alive forever.
	if e.frame > 140 {
		for i := range e.glyphs {
			e.glyphs[i].X, e.glyphs[i].Y = e.glyphs[i].TX, e.glyphs[i].TY
			e.glyphs[i].Settled = true
		}
	}
	return !e.Done()
}

func (e *SwarmTextEffect) Done() bool { return allSettled(e.glyphs) }

func (e *SwarmTextEffect) Render(r *lipgloss.Renderer, theme Theme) string {
	return renderGlyphs(r, e.glyphs, e.w, e.h, theme, lipgloss.Color(theme.Secondary))
}

// ─────────────────────────── Spotlight ───────────────────────────────────

// SpotlightTextEffect sweeps a torch beam across hidden text, revealing what
// it passes over and leaving it lit.
type SpotlightTextEffect struct {
	w, h    int
	lines   []string
	lit     []bool // per glyph
	glyphs  []glyph
	beamX   float64
	beamY   float64
	frame   int
	done    bool
}

func NewSpotlightTextEffect() *SpotlightTextEffect { return &SpotlightTextEffect{} }

func (e *SpotlightTextEffect) Name() string { return "spotlight" }

func (e *SpotlightTextEffect) Start(lines []string, w, h int) {
	e.w, e.h, e.frame, e.done = w, h, 0, false
	e.lines = lines
	e.glyphs = glyphsFromLines(lines)
	e.lit = make([]bool, len(e.glyphs))
	e.beamX, e.beamY = 0, float64(h)/2
}

func (e *SpotlightTextEffect) Step() bool {
	e.frame++
	// Sweep left→right, bobbing vertically so the beam covers the whole block.
	e.beamX += float64(e.w) / 55
	e.beamY = float64(e.h)/2 + math.Sin(float64(e.frame)*0.09)*float64(e.h)*0.42

	const radius = 9.0
	for i := range e.glyphs {
		if e.lit[i] {
			continue
		}
		g := e.glyphs[i]
		dx := (g.TX - e.beamX) * 0.5 // aspect correction
		dy := g.TY - e.beamY
		if dx*dx+dy*dy <= radius*radius {
			e.lit[i] = true
			e.glyphs[i].Settled = true
		}
	}
	if e.beamX > float64(e.w)+radius {
		// One pass is enough; light whatever the beam missed.
		for i := range e.glyphs {
			e.lit[i] = true
			e.glyphs[i].Settled = true
		}
		e.done = true
	}
	return !e.done
}

func (e *SpotlightTextEffect) Done() bool { return e.done }

func (e *SpotlightTextEffect) Render(r *lipgloss.Renderer, theme Theme) string {
	c := newCanvas(e.w, e.h)
	for i, g := range e.glyphs {
		switch {
		case e.lit[i]:
			c.set(int(g.TX), int(g.TY), g.Ch, lipgloss.Color(theme.Text))
		default:
			// Unlit glyphs sit just barely visible in the dark.
			c.set(int(g.TX), int(g.TY), g.Ch, lipgloss.Color(theme.VeryDim))
		}
	}
	// Draw the beam edge so the sweep is legible.
	if !e.done {
		bx := int(e.beamX)
		for y := 0; y < e.h; y++ {
			dy := math.Abs(float64(y) - e.beamY)
			if dy < 9 {
				c.set(bx, y, '│', lipgloss.Color(theme.Accent))
			}
		}
	}
	return c.render(r)
}

// ─────────────────────────── Blackhole ───────────────────────────────────

// BlackholeTextEffect pulls every glyph into a swirling centre, collapses,
// then explodes them back into place.
type BlackholeTextEffect struct {
	w, h   int
	glyphs []glyph
	frame  int
	phase  int // 0 = collapse, 1 = hold, 2 = explode home
}

func NewBlackholeTextEffect() *BlackholeTextEffect { return &BlackholeTextEffect{} }

func (e *BlackholeTextEffect) Name() string { return "blackhole" }

func (e *BlackholeTextEffect) Start(lines []string, w, h int) {
	e.w, e.h, e.frame, e.phase = w, h, 0, 0
	e.glyphs = glyphsFromLines(lines)
	for i := range e.glyphs {
		e.glyphs[i].X = e.glyphs[i].TX
		e.glyphs[i].Y = e.glyphs[i].TY
	}
}

func (e *BlackholeTextEffect) Step() bool {
	e.frame++
	cx, cy := float64(e.w)/2, float64(e.h)/2

	switch e.phase {
	case 0: // collapse — spiral inward
		allIn := true
		for i := range e.glyphs {
			g := &e.glyphs[i]
			dx, dy := cx-g.X, cy-g.Y
			dist := math.Hypot(dx*0.5, dy)
			if dist > 1.0 {
				allIn = false
			}
			// Radial pull plus a tangential component for the swirl.
			g.X += dx*0.13 - dy*0.10
			g.Y += dy*0.13 + dx*0.05
		}
		if allIn || e.frame > 60 {
			e.phase, e.frame = 1, 0
		}
	case 1: // hold at the singularity
		if e.frame > 8 {
			e.phase, e.frame = 2, 0
			for i := range e.glyphs {
				// Kick outward before easing home.
				e.glyphs[i].VX = (rand.Float64() - 0.5) * 6
				e.glyphs[i].VY = (rand.Float64() - 0.5) * 3
			}
		}
	case 2: // explode back into place
		for i := range e.glyphs {
			g := &e.glyphs[i]
			if g.Settled {
				continue
			}
			g.X += g.VX
			g.Y += g.VY
			g.VX *= 0.80
			g.VY *= 0.80
			dx, dy := g.TX-g.X, g.TY-g.Y
			g.VX += dx * 0.16
			g.VY += dy * 0.16
			if math.Abs(dx) < 0.4 && math.Abs(dy) < 0.4 {
				g.X, g.Y, g.Settled = g.TX, g.TY, true
			}
		}
		if e.frame > 110 {
			for i := range e.glyphs {
				e.glyphs[i].X, e.glyphs[i].Y = e.glyphs[i].TX, e.glyphs[i].TY
				e.glyphs[i].Settled = true
			}
		}
	}
	return !e.Done()
}

func (e *BlackholeTextEffect) Done() bool { return e.phase == 2 && allSettled(e.glyphs) }

func (e *BlackholeTextEffect) Render(r *lipgloss.Renderer, theme Theme) string {
	c := newCanvas(e.w, e.h)
	cx, cy := float64(e.w)/2, float64(e.h)/2
	for _, g := range e.glyphs {
		// Colour by distance from the singularity: hotter near the centre.
		d := math.Hypot((g.X-cx)*0.5, g.Y-cy)
		norm := clampF(d/(float64(e.h)/2+1), 0, 1)
		col := lipgloss.Color(theme.Text)
		if !g.Settled {
			col = heatRamp(1-norm, theme)
		}
		c.set(int(g.X+0.5), int(g.Y+0.5), g.Ch, col)
	}
	if e.phase <= 1 {
		c.setBold(int(cx), int(cy), '◉', lipgloss.Color(theme.Accent))
	}
	return c.render(r)
}

// ─────────────────────────── Particle splatter ───────────────────────────

// SplatterTextEffect blows a rendered page apart: every character becomes a
// particle with velocity and gravity. Used as an outbound page transition, so
// unlike the other text effects it never settles — it clears the screen.
type SplatterTextEffect struct {
	w, h   int
	glyphs []glyph
	frame  int
}

func NewSplatterTextEffect() *SplatterTextEffect { return &SplatterTextEffect{} }

func (e *SplatterTextEffect) Name() string { return "splatter" }

func (e *SplatterTextEffect) Start(lines []string, w, h int) {
	e.w, e.h, e.frame = w, h, 0
	e.glyphs = glyphsFromLines(lines)
	cx := float64(w) / 2
	for i := range e.glyphs {
		g := &e.glyphs[i]
		g.X, g.Y = g.TX, g.TY
		// Blow outward from the centre column, with an upward kick.
		g.VX = (g.TX - cx) * 0.045
		g.VX += (rand.Float64() - 0.5) * 1.2
		g.VY = -rand.Float64()*1.1 - 0.15
	}
}

func (e *SplatterTextEffect) Step() bool {
	e.frame++
	const gravity = 0.16
	for i := range e.glyphs {
		g := &e.glyphs[i]
		g.VY += gravity
		g.X += g.VX
		g.Y += g.VY
	}
	return !e.Done()
}

// Done once every particle has fallen off-screen, or after a hard cap so the
// transition can't stall.
func (e *SplatterTextEffect) Done() bool {
	if e.frame > 26 {
		return true
	}
	for _, g := range e.glyphs {
		if g.Y < float64(e.h) {
			return false
		}
	}
	return true
}

func (e *SplatterTextEffect) Render(r *lipgloss.Renderer, theme Theme) string {
	c := newCanvas(e.w, e.h)
	// Fade the debris out as the transition progresses.
	fade := clampF(1-float64(e.frame)/26, 0, 1)
	col := lipgloss.Color(theme.DimMid)
	switch {
	case fade > 0.66:
		col = lipgloss.Color(theme.Primary)
	case fade > 0.33:
		col = lipgloss.Color(theme.DimMid)
	default:
		col = lipgloss.Color(theme.Dim)
	}
	for _, g := range e.glyphs {
		c.set(int(g.X+0.5), int(g.Y+0.5), g.Ch, col)
	}
	return c.render(r)
}

// ─────────────────────────── Sine wave ───────────────────────────────────

// SineWave offsets each row of a block of text by a travelling sine, giving a
// floating, jelly-like sway. It's continuous rather than terminating, so it's
// applied as a filter over already-rendered content instead of being a
// TextEffect.
func SineWave(content string, frame int, amplitude float64) string {
	if amplitude <= 0 {
		return content
	}
	lines := strings.Split(content, "\n")
	out := make([]string, len(lines))
	for i, ln := range lines {
		offset := math.Sin(float64(i)*0.5+float64(frame)*0.1) * amplitude
		pad := int(offset + amplitude + 0.5) // shift into non-negative range
		if pad < 0 {
			pad = 0
		}
		out[i] = strings.Repeat(" ", pad) + ln
	}
	return strings.Join(out, "\n")
}

// NewAllTextEffects returns one of each terminating text effect, for the
// screensaver and for header reveals.
func NewAllTextEffects() []TextEffect {
	return []TextEffect{
		NewSlotTextEffect(),
		NewSwarmTextEffect(),
		NewSpotlightTextEffect(),
		NewBlackholeTextEffect(),
	}
}
