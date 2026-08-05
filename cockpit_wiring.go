package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/trafalgar-2006/ssh-portfolio/views"
)

// ─────────────────────────────────────────────────────────────────────────
// Cockpit wiring + entrance choreography
//
// The opening is a compressed cold boot that never blocks: the rain and the
// name crystallising play for about a second, then the layout is on screen and
// the content fills in around it. Any keypress jumps straight to settled, and
// a returning visitor skips it entirely.
// ─────────────────────────────────────────────────────────────────────────

const (
	// introRainTicks is how long the rain/name sequence runs before the
	// cockpit takes over. 24 ticks at 50ms ≈ 1.2s — long enough to register,
	// short enough that it never feels like waiting.
	introRainTicks = 24

	// revealTicks is how long the content takes to fill in afterwards.
	revealTicks = 26

	// ambientCycleTicks rotates the home screen's ambient effect, so a visitor
	// who lingers sees more than one.
	ambientCycleTicks = 600
)

// navItems is the rail's navigation, in the order it's drawn.
var navItems = []string{"Projects", "About", "Resume", "Contacts", "Now", "Guestbook", "Games"}

// navTargets maps a rail row to the view it opens.
var navTargets = []View{ViewProjects, ViewAbout, ViewResume, ViewContacts, ViewNow, ViewGuestbook, ViewGames}

// renderCockpit draws the home screen with its current reveal progress.
func (m Model) renderCockpit(theme views.Theme) string {
	secs := int(time.Since(m.sessionStart).Seconds())

	st := views.CockpitState{
		NavItems:   navItems,
		NavCursor:  m.activeTab,
		Session:    m.sessionID,
		Connected:  fmt.Sprintf("%02d:%02d", secs/60, secs%60),
		Ping:       m.pingMs,
		Online:     m.visitorCount,
		Build:      BuildCommit,
		LastCommit: m.lastCommit,
		Pulse:      m.livePulse,
		Tick:       m.tickCount,
		Reveal:     m.revealPct,
	}

	// Ambient panel: a live effect frame sized to fit its panel exactly.
	if m.ambientFX != nil {
		if aw, ah := views.CockpitAmbientSize(m.width, m.height); aw > 0 && ah > 0 {
			frame := m.ambientFX.Render(m.renderer, theme)
			st.Ambient = strings.Split(views.StripAnsiForTest(frame), "\n")
			st.AmbientName = m.ambientFX.Name()
		}
	}

	return views.RenderCockpit(m.renderer, m.width, m.height, st, theme)
}

// syncAmbient keeps the ambient effect sized to the panel and rotates it.
func (m *Model) syncAmbient() {
	aw, ah := views.CockpitAmbientSize(m.width, m.height)
	if aw <= 0 || ah <= 0 {
		m.ambientFX = nil // no room in this layout
		return
	}

	all := views.NewAllEffects(aw, ah)
	if len(all) == 0 {
		return
	}
	if m.ambientIdx >= len(all) {
		m.ambientIdx = 0
	}

	// Rotate on a slow cycle so a lingering visitor sees variety.
	if m.tickCount-m.ambientTick > ambientCycleTicks {
		m.ambientIdx = (m.ambientIdx + 1) % len(all)
		m.ambientTick = m.tickCount
		m.ambientFX = nil
	}

	if m.ambientFX == nil || m.ambientFX.Name() != all[m.ambientIdx].Name() {
		m.ambientFX = all[m.ambientIdx]
		m.ambientFX.Resize(aw, ah)
	} else {
		m.ambientFX.Resize(aw, ah)
	}
}

// tickIntro advances the entrance. Returns true while the intro still owns
// the screen (i.e. the rain is playing).
func (m *Model) tickIntro() bool {
	if m.introDone {
		m.revealPct = 100
		return false
	}

	// Phase 1 — rain, with the name crystallising into it.
	if m.currentView == ViewMatrix {
		if m.tickCount%2 == 0 {
			m.matrixCols = views.TickMatrixColumns(m.matrixCols, m.height)
		}
		// Lock the name in fast: the whole sequence is ~1.2s.
		if len(m.matrixPending) > 0 {
			per := len(m.matrixCells)/introRainTicks + 2
			for i := 0; i < per && len(m.matrixPending) > 0; i++ {
				pos := m.matrixPending[0]
				m.matrixPending = m.matrixPending[1:]
				m.matrixLocked[pos] = m.matrixCells[pos]
			}
		}
		if m.tickCount >= introRainTicks {
			m.currentView = ViewHome
			m.introTick = m.tickCount
			m.revealPct = 0
		}
		return true
	}

	// Phase 2 — content fills in behind an already-usable layout.
	elapsed := m.tickCount - m.introTick
	m.revealPct = clamp100(elapsed * 100 / revealTicks)
	if m.revealPct >= 100 {
		m.introDone = true
	}
	return false
}

// skipIntro jumps straight to the settled state. Any keypress calls this, so
// nothing ever stands between a visitor and the content.
func (m *Model) skipIntro() {
	if m.introDone {
		return
	}
	m.introDone = true
	m.revealPct = 100
	if m.currentView == ViewMatrix || m.currentView == ViewBoot || m.currentView == ViewAlert {
		m.currentView = ViewHome
	}
}

func clamp100(v int) int {
	if v < 0 {
		return 0
	}
	if v > 100 {
		return 100
	}
	return v
}
