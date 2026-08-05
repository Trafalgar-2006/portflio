package views

import (
	"math/rand"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// ─────────────────────────────────────────────────────────────────────────
// Animation primitives
//
// Four effects, each doing one thing, driven by a frame counter the model
// owns. Nothing here holds state — every function is a pure mapping from
// (content, frame) to a rendered string, so the same frame always renders
// identically and the whole timeline is reproducible.
// ─────────────────────────────────────────────────────────────────────────

// sizzleGlyphs are the bytes a character flickers through while decoding.
var sizzleGlyphs = []rune("@#&%*?$!/\\|<>=+~^")

// SizzleFrames is how long each character flickers before locking.
const SizzleFrames = 4

// Sizzle renders text mid-decode. Characters resolve left to right: character
// i begins flickering at frame i*stagger and locks SizzleFrames later.
//
// frame is the number of frames since the effect started. Once every character
// has locked the input is returned untouched, so callers can keep calling it
// without checking whether it's finished.
func Sizzle(s string, frame, stagger int) string {
	if frame < 0 {
		return ""
	}
	if stagger < 1 {
		stagger = 1
	}
	runes := []rune(s)

	var b strings.Builder
	b.Grow(len(s))
	for i, ch := range runes {
		start := i * stagger
		switch {
		case frame < start:
			// Not reached yet — hold the column so the line doesn't reflow.
			b.WriteRune(' ')
		case frame < start+SizzleFrames:
			if ch == ' ' {
				b.WriteRune(' ') // spaces never sizzle; they'd read as noise
			} else {
				b.WriteRune(sizzleGlyphs[rand.Intn(len(sizzleGlyphs))])
			}
		default:
			b.WriteRune(ch)
		}
	}
	return b.String()
}

// SizzleDone reports the frame at which Sizzle(s, …) has fully resolved.
func SizzleDone(s string, stagger int) int {
	if stagger < 1 {
		stagger = 1
	}
	n := len([]rune(s))
	if n == 0 {
		return 0
	}
	return (n-1)*stagger + SizzleFrames
}

// ── stream fade ──────────────────────────────────────────────────────────

// streamRamp is the colour ladder a streaming line climbs: deeply muted on
// arrival, full brightness four frames later.
func streamRamp(age int, theme Theme) lipgloss.Color {
	switch {
	case age <= 0:
		return lipgloss.Color(theme.Dim)
	case age == 1:
		return lipgloss.Color(theme.VeryDim)
	case age == 2:
		return lipgloss.Color(theme.DimMid)
	default:
		return lipgloss.Color(theme.Text)
	}
}

// StreamLines renders content arriving one row at a time, each row fading up
// from muted to full brightness over four frames.
//
// Rows already carrying their own colour (a styled heading, a status dot) are
// passed through untouched — the fade is for plain body text, and re-colouring
// a styled row would flatten it.
func StreamLines(r *lipgloss.Renderer, lines []string, frame int, theme Theme) string {
	if frame < 0 {
		return ""
	}
	var out []string
	for i, line := range lines {
		if i > frame {
			break // hasn't arrived yet
		}
		if strings.TrimSpace(line) == "" {
			out = append(out, line)
			continue
		}
		age := frame - i
		if age >= 3 || strings.Contains(line, "\x1b[") {
			out = append(out, line) // settled, or already styled
			continue
		}
		out = append(out, r.NewStyle().Foreground(streamRamp(age, theme)).Render(stripAnsi(line)))
	}
	return strings.Join(out, "\n")
}

// StreamDone reports the frame at which every line has settled.
func StreamDone(lines []string) int { return len(lines) + 3 }

// ── boot scanline ────────────────────────────────────────────────────────

// Scanline draws a single full-width rule at row `at`, on an otherwise blank
// screen of h rows. It's the first thing a visitor sees: a laser mapping out
// the space the interface is about to occupy.
func Scanline(r *lipgloss.Renderer, w, h, at int, theme Theme) string {
	if w <= 0 || h <= 0 {
		return ""
	}
	rule := r.NewStyle().Foreground(lipgloss.Color(theme.Primary)).
		Render(strings.Repeat("─", w))
	// A faint trail two rows behind, so the sweep reads as motion.
	trail := r.NewStyle().Foreground(lipgloss.Color(theme.Dim)).
		Render(strings.Repeat("─", w))

	rows := make([]string, h)
	for i := range rows {
		switch {
		case i == at:
			rows[i] = rule
		case i == at-1 || i == at-2:
			rows[i] = trail
		default:
			rows[i] = ""
		}
	}
	return strings.Join(rows, "\n")
}

// ── idle heartbeat ───────────────────────────────────────────────────────

// heartbeatSeq is the quiet pulse that proves the pipe is still open.
var heartbeatSeq = []string{".", ".", "∘", "◯", "∘", ".", "."}

// Heartbeat returns the current pulse glyph. beat advances once per cycle of
// the independent 500ms ticker, not once per render frame.
func Heartbeat(beat int) string {
	if len(heartbeatSeq) == 0 {
		return " "
	}
	i := beat % len(heartbeatSeq)
	if i < 0 {
		i += len(heartbeatSeq)
	}
	return heartbeatSeq[i]
}

// ── portrait reveal ──────────────────────────────────────────────────────

// PortraitReveal returns the first `rows` lines of the portrait, so it can be
// drawn top to bottom. Asking for more than exists returns the whole thing.
func PortraitReveal(rows int) []string {
	if rows <= 0 {
		return nil
	}
	if rows >= len(portraitArt) {
		rows = len(portraitArt)
	}
	out := make([]string, rows)
	copy(out, portraitArt[:rows])
	return out
}

// PortraitRows is the portrait's full height.
func PortraitRows() int { return len(portraitArt) }
