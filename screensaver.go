package main

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/trafalgar-2006/ssh-portfolio/views"
)

// Screensaver: a full-screen FX playground. It starts automatically after a
// long idle on the home screen, or on demand with [s]. Any key exits, except
// the ones that drive it:
//
//	n / space  next effect     p  previous effect
//	l          lock (stop auto-cycling)
//	t          theme           mouse click  interact with the effect
//
// Effects auto-advance every autoCycleTicks unless locked.
const (
	saverIdleTicks  = 900 // 45s at 50ms/tick before it kicks in on its own
	autoCycleTicks  = 400 // 20s per effect
)

// startScreensaver enters the saver, seeding the effect registry at the
// current terminal size.
func (m *Model) startScreensaver() {
	if len(m.saverFX) == 0 {
		m.saverFX = views.NewAllEffects(m.width, m.height)
	} else {
		for _, fx := range m.saverFX {
			fx.Resize(m.width, m.height)
		}
	}
	if m.saverIdx >= len(m.saverFX) {
		m.saverIdx = 0
	}
	m.saverActive = true
	m.saverTick = m.tickCount
	m.idleTicks = 0
}

func (m *Model) stopScreensaver() {
	m.saverActive = false
	m.saverLocked = false
	m.idleTicks = 0
}

func (m *Model) saverNext(delta int) {
	if len(m.saverFX) == 0 {
		return
	}
	m.saverIdx = (m.saverIdx + delta + len(m.saverFX)) % len(m.saverFX)
	m.saverFX[m.saverIdx].Resize(m.width, m.height)
	m.saverTick = m.tickCount
}

// updateScreensaver advances the active effect and handles auto-cycling.
func (m *Model) updateScreensaver() {
	if !m.saverActive || len(m.saverFX) == 0 {
		return
	}
	if m.saverIdx >= len(m.saverFX) {
		m.saverIdx = 0
	}
	m.saverFX[m.saverIdx].Step()
	if !m.saverLocked && m.tickCount-m.saverTick >= autoCycleTicks {
		m.saverNext(1)
	}
}

// handleScreensaverKey routes a key while the saver is up. It reports whether
// the saver consumed the key; anything it doesn't consume exits the saver.
func (m *Model) handleScreensaverKey(key string) bool {
	switch key {
	case "n", " ", "right", "l":
		if key == "l" {
			m.saverLocked = !m.saverLocked
			return true
		}
		m.saverNext(1)
		return true
	case "p", "left":
		m.saverNext(-1)
		return true
	case "t":
		m.themeIdx = (m.themeIdx + 1) % len(views.Themes)
		m.themeFlash = 4
		return true
	}
	return false
}

// renderScreensaver draws the active effect full-bleed with a thin chrome
// line, overlaying the effect's own output so nothing is pushed off-screen.
func (m Model) renderScreensaver(theme views.Theme) string {
	if len(m.saverFX) == 0 || m.saverIdx >= len(m.saverFX) {
		return ""
	}
	fx := m.saverFX[m.saverIdx]
	frame := fx.Render(m.renderer, theme)

	lines := strings.Split(frame, "\n")
	if len(lines) == 0 {
		return frame
	}

	r := m.renderer
	label := fmt.Sprintf(" ✦ %s  (%d/%d)", fx.Name(), m.saverIdx+1, len(m.saverFX))
	if m.saverLocked {
		label += "  [locked]"
	}
	hint := " n next · p prev · l lock · t theme · click to interact · any other key exits "

	labelS := r.NewStyle().Foreground(lipgloss.Color(theme.Accent)).Bold(true)
	hintS := r.NewStyle().Foreground(lipgloss.Color(theme.VeryDim)).Italic(true)

	// Overlay onto the first and last rows so the frame keeps its exact size.
	lines[0] = overlay(lines[0], labelS.Render(label), m.width)
	if len(lines) > 1 {
		last := len(lines) - 1
		lines[last] = overlay(lines[last], hintS.Render(hint), m.width)
	}
	return strings.Join(lines, "\n")
}

// overlay replaces the left of `base` with `text`, preserving the row's total
// display width so the frame doesn't reflow.
func overlay(base, text string, width int) string {
	tw := lipgloss.Width(text)
	if tw >= width {
		return text
	}
	// Keep the tail of the base row that the overlay doesn't cover.
	rest := lipgloss.NewStyle().MaxWidth(width).Render(base)
	restRunes := []rune(views.StripAnsiForTest(rest))
	if len(restRunes) > tw {
		return text + string(restRunes[tw:])
	}
	return text
}
