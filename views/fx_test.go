package views

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

// Every screen effect must survive any terminal size, and every frame it
// renders must be exactly w columns by h rows — anything else corrupts the
// terminal or scrolls the screen.
func TestScreenEffectsFitAtEverySize(t *testing.T) {
	r := lipgloss.DefaultRenderer()
	sizes := [][2]int{{0, 0}, {1, 1}, {2, 3}, {10, 5}, {40, 12}, {80, 24}, {120, 40}, {200, 60}, {35, 70}}

	for _, sz := range sizes {
		w, h := sz[0], sz[1]
		for _, fx := range NewAllEffects(w, h) {
			fx, w, h := fx, w, h
			t.Run(fx.Name(), func(t *testing.T) {
				defer func() {
					if rec := recover(); rec != nil {
						t.Fatalf("%s panicked at %dx%d: %v", fx.Name(), w, h, rec)
					}
				}()
				for i := 0; i < 40; i++ {
					fx.Step()
					out := fx.Render(r, ThemeDracula)
					if w == 0 || h == 0 {
						continue // nothing to draw
					}
					lines := strings.Split(out, "\n")
					if len(lines) != h {
						t.Fatalf("%s at %dx%d frame %d: %d rows, want %d", fx.Name(), w, h, i, len(lines), h)
					}
					for y, ln := range lines {
						if got := lipgloss.Width(ln); got != w {
							t.Fatalf("%s at %dx%d frame %d row %d: %d cols, want %d",
								fx.Name(), w, h, i, y, got, w)
						}
					}
				}
			})
		}
	}
}

// Interact must be safe for any coordinate, including well off-screen.
func TestScreenEffectsInteractSafely(t *testing.T) {
	r := lipgloss.DefaultRenderer()
	for _, fx := range NewAllEffects(60, 20) {
		fx := fx
		defer func() {
			if rec := recover(); rec != nil {
				t.Fatalf("%s panicked on Interact: %v", fx.Name(), rec)
			}
		}()
		for _, p := range [][2]int{{0, 0}, {59, 19}, {-5, -5}, {1000, 1000}, {30, -1}, {-1, 10}} {
			fx.Interact(p[0], p[1])
			fx.Step()
			fx.Render(r, ThemeDracula)
		}
	}
}

// Resizing mid-run must not corrupt an effect's state.
func TestScreenEffectsSurviveResize(t *testing.T) {
	r := lipgloss.DefaultRenderer()
	for _, fx := range NewAllEffects(80, 24) {
		fx := fx
		defer func() {
			if rec := recover(); rec != nil {
				t.Fatalf("%s panicked across resize: %v", fx.Name(), rec)
			}
		}()
		for _, sz := range [][2]int{{120, 40}, {20, 8}, {200, 60}, {1, 1}, {80, 24}} {
			fx.Resize(sz[0], sz[1])
			for i := 0; i < 8; i++ {
				fx.Step()
				out := fx.Render(r, ThemeDracula)
				if n := len(strings.Split(out, "\n")); n != sz[1] {
					t.Fatalf("%s after resize to %dx%d: %d rows", fx.Name(), sz[0], sz[1], n)
				}
			}
		}
	}
}

// Every effect must render in every theme without an unset colour.
func TestScreenEffectsAllThemes(t *testing.T) {
	r := lipgloss.DefaultRenderer()
	for _, th := range Themes {
		for _, fx := range NewAllEffects(60, 20) {
			for i := 0; i < 5; i++ {
				fx.Step()
			}
			if out := fx.Render(r, th); out == "" {
				t.Errorf("%s rendered empty under theme %s", fx.Name(), th.Name)
			}
		}
	}
}

// ─────────────────────────── text effects ────────────────────────────────

var sampleLines = []string{
	"MOHITH AKSHAY",
	"  building things that think",
	"    ssh mohith.is-a.dev",
}

// Every terminating text effect must actually terminate, and must land every
// glyph on its home position when it does.
func TestTextEffectsTerminateAndSettle(t *testing.T) {
	r := lipgloss.DefaultRenderer()
	for _, fx := range NewAllTextEffects() {
		fx := fx
		t.Run(fx.Name(), func(t *testing.T) {
			fx.Start(sampleLines, 60, 12)
			frames := 0
			for fx.Step() {
				fx.Render(r, ThemeDracula)
				frames++
				if frames > 500 {
					t.Fatalf("%s never terminated (>500 frames)", fx.Name())
				}
			}
			if !fx.Done() {
				t.Errorf("%s stopped stepping but reports not done", fx.Name())
			}
			// Settled output must contain the original text.
			out := stripAnsi(fx.Render(r, ThemeDracula))
			for _, want := range sampleLines {
				if trimmed := strings.TrimSpace(want); trimmed != "" && !strings.Contains(out, trimmed) {
					t.Errorf("%s settled output is missing %q", fx.Name(), trimmed)
				}
			}
		})
	}
}

// Text effects must be safe at degenerate sizes and with degenerate input.
func TestTextEffectsEdgeCases(t *testing.T) {
	r := lipgloss.DefaultRenderer()
	inputs := [][]string{
		nil,
		{},
		{""},
		{"    "},
		{strings.Repeat("x", 500)},
		sampleLines,
	}
	for _, fx := range NewAllTextEffects() {
		for _, in := range inputs {
			for _, sz := range [][2]int{{0, 0}, {1, 1}, {5, 2}, {80, 24}} {
				fx, in, sz := fx, in, sz
				func() {
					defer func() {
						if rec := recover(); rec != nil {
							t.Fatalf("%s panicked (input %d lines, %dx%d): %v",
								fx.Name(), len(in), sz[0], sz[1], rec)
						}
					}()
					fx.Start(in, sz[0], sz[1])
					for i := 0; i < 300 && fx.Step(); i++ {
						fx.Render(r, ThemeDracula)
					}
				}()
			}
		}
	}
}

// The splatter transition must always finish — a stalled transition would
// leave the UI stuck mid-navigation.
func TestSplatterAlwaysCompletes(t *testing.T) {
	r := lipgloss.DefaultRenderer()
	fx := NewSplatterTextEffect()
	fx.Start(sampleLines, 80, 24)
	frames := 0
	for fx.Step() {
		fx.Render(r, ThemeDracula)
		frames++
		if frames > 200 {
			t.Fatal("splatter transition never completed")
		}
	}
	if frames == 0 {
		t.Error("splatter completed instantly — no visible transition")
	}
}

// SineWave must preserve every line and never produce negative padding.
func TestSineWavePreservesLines(t *testing.T) {
	in := "alpha\nbeta\ngamma\ndelta"
	for frame := 0; frame < 40; frame++ {
		out := SineWave(in, frame, 3)
		if got, want := len(strings.Split(out, "\n")), 4; got != want {
			t.Fatalf("frame %d: %d lines, want %d", frame, got, want)
		}
		for _, ln := range strings.Split(out, "\n") {
			if strings.TrimSpace(ln) == "" {
				t.Fatalf("frame %d: a line was blanked", frame)
			}
		}
	}
	// Zero amplitude must be a no-op.
	if got := SineWave(in, 5, 0); got != in {
		t.Error("zero amplitude changed the content")
	}
}

// The canvas must clip rather than panic, and group runs correctly.
func TestCanvasClipsAndRenders(t *testing.T) {
	c := newCanvas(10, 3)
	for _, p := range [][2]int{{-1, 0}, {0, -1}, {10, 0}, {0, 3}, {999, 999}, {-999, -999}} {
		c.set(p[0], p[1], 'X', "#FF0000") // must not panic
	}
	c.text(6, 1, "OVERFLOWING", "#00FF00") // must clip at the edge
	out := c.render(lipgloss.DefaultRenderer())
	lines := strings.Split(out, "\n")
	if len(lines) != 3 {
		t.Fatalf("canvas emitted %d rows, want 3", len(lines))
	}
	for i, ln := range lines {
		if got := lipgloss.Width(ln); got != 10 {
			t.Errorf("row %d is %d cols, want 10", i, got)
		}
	}
	if !strings.Contains(stripAnsi(out), "OVER") {
		t.Error("clipped text lost its visible prefix")
	}
}

// A zero-size canvas must render empty rather than panicking.
func TestCanvasZeroSize(t *testing.T) {
	if got := newCanvas(0, 0).render(lipgloss.DefaultRenderer()); got != "" {
		t.Errorf("zero canvas rendered %q", got)
	}
	newCanvas(-5, -5).render(lipgloss.DefaultRenderer())
}
