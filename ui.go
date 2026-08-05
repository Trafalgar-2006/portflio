package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/trafalgar-2006/ssh-portfolio/views"
)

// ─────────────────────────────────────────────────────────────────────────
// UI state machine
//
//	STATE 0  BOOT     scanline sweep → decoder sizzle → portrait draw
//	STATE 1  IDLE     heartbeat on its own slow ticker; hover on nav
//	STATE 2  SHADE    tab switch: instant clear, content streams in
//
// The model owns a single frame counter. Every phase is a pure function of
// where that counter sits relative to the phase boundaries below, so the
// timeline is deterministic and a given frame always renders identically.
// ─────────────────────────────────────────────────────────────────────────

type uiState int

const (
	stateBoot uiState = iota
	stateIdle
	stateShade
)

// Phase boundaries, in 50ms frames.
//
//	 0– 6   scanline sweeps the full height          (0–300ms)
//	 6–12   rules snap in, text decodes              (300–600ms)
//	12–18   portrait draws line by line              (600–900ms)
const (
	bootScanEnd     = 6
	bootSizzleEnd   = 12
	bootPortraitEnd = 18

	// shadeFrames is how long an incoming page takes to stream.
	shadeFrames = 14

	// heartbeatEvery is the independent 500ms pulse, in 50ms frames.
	heartbeatEvery = 10
)

// Sentinels for "no animation running" and "pointer not over a nav item".
//
// These are deliberately not zero: tickCount also starts at zero, so a
// zero-valued shadeStart would read as "a page re-shade began on frame 0" and
// the first frame would render a single streaming line instead of the page.
const (
	noShade = -1
	noHover = -1
)

// navLabels is the primary navigation, as it reads in the hero.
var navLabels = []string{"Projects", "About", "Resume", "Guestbook", "Games", "Quit"}

// navView maps a nav slot to the view it opens. Quit is handled separately.
var navView = []View{ViewProjects, ViewAbout, ViewResume, ViewGuestbook, ViewGames, ViewHome}

// uiPhase reports which state the interface is in.
func (m Model) uiPhase() uiState {
	if !m.bootDone {
		return stateBoot
	}
	if m.shadeStart != noShade && m.tickCount-m.shadeStart < shadeFrames {
		return stateShade
	}
	return stateIdle
}

// bootFrame is how many frames the boot sequence has been running.
func (m Model) bootFrame() int { return m.tickCount - m.bootStart }

// tickUI advances the boot timeline. Returns true while boot owns the screen
// exclusively (the scanline phase, before any layout exists).
func (m *Model) tickUI() bool {
	if m.bootDone {
		return false
	}
	if m.bootFrame() >= bootPortraitEnd {
		m.finishBoot()
		return false
	}
	return m.bootFrame() < bootScanEnd
}

// finishBoot jumps to the settled state. Any keypress calls this.
func (m *Model) finishBoot() {
	if m.bootDone {
		return
	}
	m.bootDone = true
	if m.currentView < ViewHome {
		m.currentView = ViewHome
	}
}

// beginShade starts the page re-shade: the content pane clears instantly and
// the incoming rows stream in.
func (m *Model) beginShade() { m.shadeStart = m.tickCount }

// heroState assembles the header's animation state for this frame.
func (m Model) heroState() views.HeroState {
	st := views.HeroState{
		Nav:          navLabels,
		NavCursor:    m.activeTab,
		NavHover:     m.navHover,
		PortraitRows: views.PortraitRows(),
		MaxRows:      portraitCap(m.height),
		SizzleFrame:  -1,
	}

	if !m.bootDone {
		f := m.bootFrame()
		switch {
		case f < bootScanEnd:
			// Nothing yet — the scanline is still mapping the space.
			st.PortraitRows = 0
			st.SizzleFrame = 0
		case f < bootSizzleEnd:
			// Text decodes; portrait hasn't started.
			st.PortraitRows = 0
			st.SizzleFrame = (f - bootScanEnd) * 3
		default:
			// Portrait draws line by line.
			prog := f - bootSizzleEnd
			total := bootPortraitEnd - bootSizzleEnd
			st.PortraitRows = views.PortraitRows() * prog / maxI(total, 1)
			st.SizzleFrame = -1
		}
	}
	return st
}

// contentLines builds the body for the current view as flat rows, ready to
// stream. One column, always.
func (m Model) contentLines(theme views.Theme) []string {
	w := m.width
	// The keymap takes over the content pane rather than covering the screen,
	// so the header stays put and you never lose your place.
	if m.helpOpen {
		return views.BlockHelp(m.renderer, w, theme)
	}
	switch m.currentView {
	case ViewHome, ViewProjects:
		return views.BlockProjects(m.renderer, w, m.projectCursor, theme)
	case ViewAbout:
		return views.BlockAbout(m.renderer, w, theme)
	case ViewResume:
		return views.BlockResume(m.renderer, w, theme)
	case ViewContacts:
		return views.BlockContacts(m.renderer, w, m.contactsCopyMode, theme)
	case ViewNow:
		return views.BlockNow(m.renderer, w, buildDateLabel(), theme)
	case ViewGuestbook:
		return strings.Split(views.RenderGuestbook(m.renderer, w, m.height,
			m.guestEntries, m.guestInput, int(m.visitorCount),
			m.tickCount%14 < 7, theme), "\n")
	case ViewTimeline:
		return strings.Split(views.RenderTimeline(m.renderer, w, m.height,
			m.timelineCursor, views.Commits(), theme), "\n")
	case ViewAdmin:
		if m.isAdmin {
			return strings.Split(views.RenderAdmin(m.renderer, w, m.height,
				collectAdminStats(), theme), "\n")
		}
		return strings.Split(views.RenderAdminDenied(m.renderer, theme), "\n")
	case ViewNeofetch:
		info := m.client
		info.SessionID = m.sessionID
		secs := int(time.Since(m.sessionStart).Seconds())
		info.Connected = fmt.Sprintf("%02d:%02d", secs/60, secs%60)
		if info.Width == 0 {
			info.Width, info.Height = m.width, m.height
		}
		return strings.Split(views.RenderNeofetch(m.renderer, w, m.height, info, theme), "\n")
	case ViewGames:
		if len(m.games) > 0 && m.gameIdx < len(m.games) {
			return strings.Split(m.games[m.gameIdx].Render(m.renderer, theme), "\n")
		}
	}
	return nil
}

// renderUI composes the whole frame.
func (m Model) renderUI(theme views.Theme) string {
	// STATE 0, phase I — the laser sweep, on an otherwise blank screen.
	if !m.bootDone && m.bootFrame() < bootScanEnd {
		at := m.bootFrame() * m.height / maxI(bootScanEnd, 1)
		return views.Scanline(m.renderer, m.width, m.height, at, theme)
	}

	header := views.RenderHero(m.renderer, m.width, m.heroState(), theme)

	// Content is withheld until the header has decoded — the eye should land
	// on the identity first, then the work.
	var content string
	if m.bootDone {
		lines := m.contentLines(theme)
		if m.uiPhase() == stateShade {
			// STATE 2 — stream in with the colour ramp.
			content = views.StreamLines(m.renderer, lines, m.tickCount-m.shadeStart, theme)
		} else {
			content = strings.Join(lines, "\n")
		}
	}

	secs := int(time.Since(m.sessionStart).Seconds())
	return views.Compose(m.renderer, m.width, m.height, views.ScreenState{
		Header:  header,
		Content: content,
		Scroll:  m.scrollY,
		Beat:    m.tickCount / heartbeatEvery,
		Telemetry: views.Telemetry{
			Latency: m.pingMs,
			Online:  m.visitorCount,
			Session: m.sessionID,
			Uptime:  fmt.Sprintf("%02d:%02d", secs/60, secs%60),
		},
		Hint: "? help   / search   ↑↓ move   ⏎ open   t theme",
	}, theme)
}

// portraitCap is the tallest the portrait may be for a given terminal.
// Roughly half the window, so the content pane always has real room, floored
// so the face is never reduced to a smear and capped at the art's full height.
func portraitCap(h int) int {
	c := h / 2
	if c < 8 {
		c = 8
	}
	if full := views.PortraitRows(); c > full {
		c = full
	}
	return c
}

// contentPaneHeight is how many rows the body pane gets right now.
func (m Model) contentPaneHeight() int {
	header := views.RenderHero(m.renderer, m.width, m.heroState(), views.Themes[m.themeIdx])
	rows := len(strings.Split(header, "\n"))
	return views.ContentHeight(m.height, rows)
}

// maxContentScroll is the furthest the body can scroll.
func (m Model) maxContentScroll(theme views.Theme) int {
	n := len(m.contentLines(theme)) - m.contentPaneHeight()
	if n < 0 {
		return 0
	}
	return n
}

func maxI(a, b int) int {
	if a > b {
		return a
	}
	return b
}
