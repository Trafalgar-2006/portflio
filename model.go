package main

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/trafalgar-2006/ssh-portfolio/views"
)

// Build info — injected via ldflags at build time
var (
	BuildVersion = "dev"
	BuildCommit  = "unknown"
	BuildDate    = "unknown"
)

// buildDateLabel renders BuildDate as "January 2006" for the /now freshness
// stamp. Returns "" when the binary wasn't stamped (plain `go build`), so the
// page omits the line rather than showing a wrong or placeholder date.
func buildDateLabel() string {
	for _, layout := range []string{time.RFC3339, "2006-01-02T15:04:05Z", "2006-01-02"} {
		if t, err := time.Parse(layout, BuildDate); err == nil {
			return t.Format("January 2006")
		}
	}
	return ""
}

type View int

const (
	ViewMatrix View = iota // NEW: matrix rain pre-splash
	ViewBoot
	ViewAlert
	ViewHome
	ViewProjects
	ViewAbout
	ViewContacts
	ViewResume
	ViewNow
	ViewGames
	ViewNeofetch
	ViewGuestbook
	ViewAdmin
	ViewTimeline
)

var tabNames = []string{"Projects", "About", "Contacts", "Resume", "/now"}

type tickMsg time.Time

// frameInterval is the animation clock. 50ms = 20fps, which is the sweet spot
// over SSH: fast enough to read as smooth, slow enough that a full-screen
// effect on a 200x60 terminal doesn't saturate a slow link.
//
// Effects that don't need every frame (matrix rain) already sub-sample it.
const frameInterval = 50 * time.Millisecond

func tickCmd() tea.Cmd {
	return tea.Tick(frameInterval, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

// defaultWidth/Height are the assumed terminal size until the client reports
// a real one — and the fallback when it reports a nonsensical 0x0.
const (
	defaultWidth  = 80
	defaultHeight = 24
)

const numStars = 8

// StarState tracks per-star independent twinkle timing
type StarState struct {
	Bright   bool
	FlipAt   int // tickCount when this star should next flip
}

type Model struct {
	renderer  *lipgloss.Renderer
	width     int
	height    int
	quitting  bool
	tickCount int

	// Matrix rain animation (pre-boot)
	matrixCols    []views.MatrixColumn
	matrixLocked  map[[2]int]rune
	matrixPending [][2]int
	matrixCells   map[[2]int]rune // banner cell lookup, cached per size (not per tick)
	matrixPhase     int
	matrixPhaseTick int // tickCount when the current matrix phase began
	matrixNameX     int
	matrixNameY     int

	// Boot animation
	bootLines     []views.BootLine
	bootSchedule  []int
	bootVisible   int
	bootStartTick int

	// Alert animation
	alertPhase     int
	alertPhaseTick int

	// Home / Splash
	currentView  View
	activeTab    int
	revealIdx    int
	glitchFrames int
	glitchRunes  [][]rune
	taglineIdx   int
	taglineDone  bool
	cursorBlink  bool
	cursorLeft   int
	lastCommit   string

	// Independent star twinkle
	stars [numStars]StarState

	// CRT scanline sweep
	scanlineY int // -1 = disabled, 0..height = current line

	// Idle ambient glitch
	idleTicks  int  // resets on any keypress
	idleGlitch bool // true for exactly 1 tick (one-frame flicker)

	// Session info
	sessionID    string
	sessionStart time.Time

	// Theme
	themeIdx   int // index into views.Themes
	themeFlash int // counts down from 4, triggers flash overlay

	// Viewport scroll offset for the current view (reset on navigation)
	scrollY int

	// Wipe transition
	wipePhase   int
	wipeLines   int
	pendingView View
	pendingTab  int

	// Projects
	projectCursor  int
	projectScroll  int
	projectsReveal int
	tagPopReveal   int
	lastCursor     int
	livePulse      bool

	// Projects — smooth highlight + momentum
	highlightY float64 // visual lerp position of the selection bar
	velocity   float64 // scroll momentum (decays each tick)

	// Projects — decrypt reveal on open
	decryptIdx   int      // chars revealed so far in description
	decryptRunes []rune   // scrambled desc runes, resolved left-to-right

	// Vim number prefix buffer (e.g. "3" before j)
	numBuf string

	// Visitor counter + network ping display
	visitorCount int64
	pingMs       int // fake jittering ping in ms
	pingJitter   int // countdown until next ping update

	// Contacts
	contactsReveal  int
	sshFlash        int
	contactsCopyMode bool

	// Quit confirm (double-press q within 2s)
	quitPending     bool
	quitPendingTick int

	// Konami easter egg: ↑↑↓↓←→←→ba
	konamiSeq  []string
	konamiDone bool
	konamiTick int

	// Portrait shimmer (home screen)
	portraitShimRow   int // -1 = none
	portraitShimFrame int

	// Command palette
	cmdActive bool
	cmdQuery  string
	cmdSelIdx int

	// Exit animation
	exitPhase int
	exitTick  int

	// Cursor ghost trail (projects)
	ghostCursor1 int
	ghostFade1   int
	ghostCursor2 int
	ghostFade2   int

	// Screensaver — cycles the full-screen FX registry
	saverActive bool
	saverFX     []views.Effect
	saverIdx    int
	saverTick   int // tick the current effect started on
	saverLocked bool // true when the user picked an effect explicitly

	// Particle splatter page transition. It fires once per session — see
	// startWipe for why.
	splatter     *views.SplatterTextEffect
	splatterOn   bool
	splatterUsed bool

	// Header text effect (slot machine / swarm / etc. on section titles)
	headerFX   views.TextEffect
	headerOn   bool
	headerFor  View

	// Sine wave distortion toggle
	waveOn bool

	// Mini-games
	games   []views.Game
	gameIdx int

	// Client/session facts, used by the neofetch greeting and admin view
	client views.ClientInfo
	// directRoute is the view requested via `ssh host <command>`, if any
	directRoute string

	// Guestbook
	guestInput   views.GuestbookInput
	guestEntries []views.GuestEntry
	guestOnline  int
	guestNotify  <-chan struct{} // broadcast from other sessions

	// Admin dashboard (gated on an operator SSH key)
	isAdmin bool

	// Git time-travel timeline
	timelineCursor int

	// Memoised body for static views (see frameCache)
	cache *frameCache
}

func NewModel(r *lipgloss.Renderer) Model {
	if r == nil {
		r = lipgloss.DefaultRenderer()
	}
	lines, sid := views.NewBootSequence()
	schedule := make([]int, len(lines))
	t := 0
	for i, l := range lines {
		schedule[i] = t
		t += l.DelayTicks
	}

	// Matrix: initialise with default terminal size; resized on first WindowSizeMsg
	w, h := defaultWidth, defaultHeight
	nameX, nameY := views.MatrixNameOrigin(w, h)
	allCells := views.ComputeNameCells(nameX, nameY, views.NameBannerLines())
	pending := make([][2]int, 0, len(allCells))
	for pos := range allCells {
		pending = append(pending, pos)
	}
	// Shuffle pending so cells lock in random order
	rand.Shuffle(len(pending), func(i, j int) { pending[i], pending[j] = pending[j], pending[i] })

	// Initialise stars with random independent flip times
	var stars [numStars]StarState
	for i := range stars {
		stars[i] = StarState{
			Bright: rand.Intn(2) == 0,
			FlipAt: rand.Intn(30) + 10, // 0.5s–1.5s first flip
		}
	}

	return Model{
		renderer:          r,
		width:             w,
		height:            h,
		currentView:       ViewMatrix,
		matrixCols:        views.NewMatrixColumns(w, h),
		matrixLocked:      make(map[[2]int]rune),
		matrixPending:     pending,
		matrixCells:       allCells,
		matrixNameX:       nameX,
		matrixNameY:       nameY,
		bootLines:         lines,
		bootSchedule:      schedule,
		bootVisible:       0,
		cursorLeft:        6,
		sessionID:         sid,
		sessionStart:      time.Now(),
		stars:             stars,
		scanlineY:         -1,
		pingMs:            12 + rand.Intn(9),
		pingJitter:        8,
		portraitShimRow:   -1,
		cache:             &frameCache{},
	}
}

// guestbookMsg signals that another session posted a message.
type guestbookMsg struct{}

// waitForGuestbook blocks on the guestbook notification channel and turns the
// next broadcast into a message. Re-armed after each delivery, so a session
// keeps receiving updates for as long as it's connected.
func waitForGuestbook(ch <-chan struct{}) tea.Cmd {
	if ch == nil {
		return nil
	}
	return func() tea.Msg {
		if _, ok := <-ch; !ok {
			return nil // unsubscribed
		}
		return guestbookMsg{}
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(tickCmd(), fetchCommit(), fetchHistory(), waitForGuestbook(m.guestNotify))
}

// applyDirectRoute honours `ssh mohith.is-a.dev <command>` by skipping the
// intro and opening the requested screen immediately.
func (m *Model) applyDirectRoute() {
	if m.directRoute == "" {
		return
	}
	route := m.directRoute
	m.directRoute = "" // one-shot

	// Jump past matrix/boot/alert straight to content.
	m.currentView = ViewHome
	m.revealIdx = views.BannerLines()
	m.taglineIdx = len([]rune(views.TaglineText))
	m.taglineDone = true
	m.cursorLeft = 0

	switch route {
	case "projects":
		m.currentView, m.activeTab = ViewProjects, 0
		m.projectsReveal, m.tagPopReveal = 999, 999
	case "about":
		m.currentView, m.activeTab = ViewAbout, 1
	case "contacts":
		m.currentView, m.activeTab = ViewContacts, 2
		m.contactsReveal = 999
	case "resume":
		m.currentView, m.activeTab = ViewResume, 3
	case "now":
		m.currentView, m.activeTab = ViewNow, 4
	case "neofetch", "help":
		m.currentView = ViewNeofetch
	case "snake":
		m.games = views.NewAllGames(m.width, m.height)
		m.gameIdx, m.currentView = 0, ViewGames
	case "tetris":
		m.games = views.NewAllGames(m.width, m.height)
		m.gameIdx, m.currentView = 1, ViewGames
	case "fx":
		m.startScreensaver()
	case "guestbook":
		m.currentView = ViewGuestbook
	case "admin":
		if m.isAdmin {
			m.currentView = ViewAdmin
		} else {
			m.currentView = ViewNeofetch
		}
	}
}

// rebuildMatrix re-seeds the rain columns and the crystallising name origin for
// the current terminal size, preserving how far the reveal has already got.
func (m *Model) rebuildMatrix() {
	progress := 0.0
	if total := len(m.matrixPending) + len(m.matrixLocked); total > 0 {
		progress = float64(len(m.matrixLocked)) / float64(total)
	}

	m.matrixCols = views.NewMatrixColumns(m.width, m.height)
	m.matrixNameX, m.matrixNameY = views.MatrixNameOrigin(m.width, m.height)

	cells := views.ComputeNameCells(m.matrixNameX, m.matrixNameY, views.NameBannerLines())
	m.matrixCells = cells
	positions := make([][2]int, 0, len(cells))
	for pos := range cells {
		positions = append(positions, pos)
	}
	rand.Shuffle(len(positions), func(i, j int) { positions[i], positions[j] = positions[j], positions[i] })

	// Re-lock the same fraction that was already revealed so the animation
	// doesn't visibly restart when someone resizes mid-intro.
	lockCount := int(progress * float64(len(positions)))
	m.matrixLocked = make(map[[2]int]rune, lockCount)
	for i := 0; i < lockCount && i < len(positions); i++ {
		m.matrixLocked[positions[i]] = cells[positions[i]]
	}
	if lockCount > len(positions) {
		lockCount = len(positions)
	}
	m.matrixPending = positions[lockCount:]
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		// Some clients (and non-interactive `ssh -tt` invocations) report a
		// 0x0 window. Taking that literally collapses the viewport to one row,
		// so fall back to a sane default instead of believing it.
		w, h := msg.Width, msg.Height
		if w <= 0 {
			w = defaultWidth
		}
		if h <= 0 {
			h = defaultHeight
		}
		if w == m.width && h == m.height {
			return m, nil
		}
		m.width = w
		m.height = h
		// The rain and the name it crystallises into were sized for the initial
		// 80x24 guess. Rebuild them, or wide terminals get rain in the leftmost
		// 80 columns and an off-centre name.
		if m.currentView == ViewMatrix {
			m.rebuildMatrix()
		}
		m.scrollBy(0) // re-clamp: a taller terminal can shrink maxScroll
		return m, nil

	case tea.MouseMsg:
		// Mouse clicks drive the interactive sandboxes (sand, life, fire…).
		if m.saverActive && len(m.saverFX) > 0 && m.saverIdx < len(m.saverFX) {
			if msg.Action == tea.MouseActionPress || msg.Action == tea.MouseActionMotion {
				m.saverFX[m.saverIdx].Interact(msg.X, msg.Y)
			}
		}
		return m, nil

	case tea.KeyMsg:
		// Screensaver swallows keys it uses; anything else dismisses it.
		if m.saverActive {
			if m.handleScreensaverKey(msg.String()) {
				return m, nil
			}
			m.stopScreensaver()
			return m, nil
		}

		// The guestbook compose box swallows text input while active.
		if m.currentView == ViewGuestbook && m.guestInput.Active {
			switch k := msg.String(); k {
			case "esc":
				m.guestInput.Active = false
				m.guestInput.Err = ""
			case "tab", "shift+tab":
				m.guestInput.Field = 1 - m.guestInput.Field
			case "enter":
				if strings.TrimSpace(m.guestInput.Message) == "" {
					m.guestInput.Err = "message can't be empty"
					break
				}
				if _, ok := TheGuestbook.Post(m.guestInput.Name, m.guestInput.Message, m.sessionID); ok {
					m.guestInput = views.GuestbookInput{JustPosted: true}
					m.guestEntries = TheGuestbook.Entries()
					m.scrollBy(1 << 20) // jump to the newest message
				} else {
					m.guestInput.Err = "message was empty after trimming"
				}
			case "backspace":
				if m.guestInput.Field == 0 {
					m.guestInput.Name = dropLastRune(m.guestInput.Name)
				} else {
					m.guestInput.Message = dropLastRune(m.guestInput.Message)
				}
			case "space":
				m.appendGuestRune(" ")
			default:
				if len([]rune(k)) == 1 {
					m.appendGuestRune(k)
				}
			}
			m.idleTicks = 0
			return m, nil
		}

		// Reading the guestbook: [i] opens the compose box.
		if m.currentView == ViewGuestbook && msg.String() == "i" {
			m.guestInput.Active = true
			m.guestInput.JustPosted = false
			m.guestInput.Err = ""
			if m.guestInput.Name == "" {
				m.guestInput.Name = m.client.User
			}
			return m, nil
		}

		// An active game claims movement keys; esc still exits to home.
		if m.currentView == ViewGames && len(m.games) > 0 && m.gameIdx < len(m.games) {
			k := msg.String()
			if k == "esc" || k == "ctrl+c" || k == "q" {
				m.startWipe(ViewHome, m.activeTab)
				return m, nil
			}
			if k == "t" {
				m.themeIdx = (m.themeIdx + 1) % len(views.Themes)
				m.themeFlash = 4
				return m, nil
			}
			if m.games[m.gameIdx].Key(k) {
				m.idleTicks = 0
				return m, nil
			}
		}

		// Track key for Konami sequence
		m.trackKonami(msg.String())

		// Command palette intercepts most keys when active.
		// Only arrows/ctrl-n/ctrl-p navigate — j and k must stay typable, or
		// queries containing them ("projects") can never be entered.
		if m.cmdActive {
			switch msg.String() {
			case "esc", "ctrl+c":
				m.cmdActive = false
				m.cmdQuery = ""
				m.cmdSelIdx = 0
			case "enter":
				m.applyCmdSelection()
			case "up", "ctrl+p":
				if m.cmdSelIdx > 0 { m.cmdSelIdx-- }
			case "down", "ctrl+n":
				if m.cmdSelIdx < m.cmdMatchCount()-1 { m.cmdSelIdx++ }
			case "backspace":
				if len(m.cmdQuery) > 0 {
					runes := []rune(m.cmdQuery)
					m.cmdQuery = string(runes[:len(runes)-1])
					m.cmdSelIdx = 0
				}
			case "space":
				m.cmdQuery += " "
				m.cmdSelIdx = 0
			default:
				if len([]rune(msg.String())) == 1 {
					m.cmdQuery += msg.String()
					m.cmdSelIdx = 0
				}
			}
			return m, nil
		}

		// Reset idle counter on any keypress
		m.idleTicks = 0
		switch msg.String() {
		case "ctrl+c":
			m.exitPhase = 1
			m.exitTick = m.tickCount
			return m, nil

		case "/":
			// Open command palette from any content screen
			if m.currentView >= ViewHome {
				m.cmdActive = true
				m.cmdQuery = ""
				m.cmdSelIdx = 0
			}
			return m, nil

		case "t":
			// Cycle theme with flash
			m.themeIdx = (m.themeIdx + 1) % len(views.Themes)
			m.themeFlash = 4
			return m, nil

		case "c":
			// Toggle copy-friendly mode in contacts
			if m.currentView == ViewContacts {
				m.contactsCopyMode = !m.contactsCopyMode
			}
			return m, nil

		case "s":
			// Screensaver / FX playground
			if m.currentView >= ViewHome {
				m.startScreensaver()
			}
			return m, nil

		case "w":
			// Sine-wave distortion toggle
			if m.currentView >= ViewHome {
				m.waveOn = !m.waveOn
			}
			return m, nil

		case "ctrl+k":
			// Same palette as "/", matching the common editor binding.
			if m.currentView >= ViewHome {
				m.cmdActive = true
				m.cmdQuery = ""
				m.cmdSelIdx = 0
			}
			return m, nil

		case "q":
			if m.currentView == ViewHome {
				if m.quitPending && m.tickCount-m.quitPendingTick <= 80 {
					// Confirmed — play exit animation
					m.exitPhase = 1
					m.exitTick = m.tickCount
					m.quitPending = false
					return m, nil
				}
				m.quitPending = true
				m.quitPendingTick = m.tickCount
				return m, nil
			}
			m.startWipe(ViewHome, m.activeTab)
			return m, nil

		case "esc":
			if m.currentView != ViewHome && m.currentView != ViewBoot && m.currentView != ViewAlert {
				m.contactsCopyMode = false
				m.startWipe(ViewHome, m.activeTab)
				return m, nil
			}
			if m.currentView == ViewHome {
				if m.quitPending && m.tickCount-m.quitPendingTick <= 80 {
					m.exitPhase = 1
					m.exitTick = m.tickCount
					m.quitPending = false
					return m, nil
				}
			}
			return m, nil

		case "left", "h":
			if m.currentView == ViewTimeline {
				if m.timelineCursor < len(views.Commits())-1 {
					m.timelineCursor++ // left = further back in time
				}
				return m, nil
			}
			if m.currentView == ViewHome {
				m.quitPending = false
				m.activeTab--
				if m.activeTab < 0 {
					m.activeTab = len(tabNames) - 1
				}
			}
			return m, nil

		case "right", "l":
			if m.currentView == ViewTimeline {
				if m.timelineCursor > 0 {
					m.timelineCursor-- // right = forward toward today
				}
				return m, nil
			}
			if m.currentView == ViewHome {
				m.quitPending = false
				m.activeTab++
				if m.activeTab >= len(tabNames) {
					m.activeTab = 0
				}
			}
			return m, nil

		case "up", "k":
			if m.currentView == ViewProjects {
				steps := m.consumeNum(1)
				prev := m.projectCursor
				m.projectCursor -= steps
				if m.projectCursor < 0 {
					m.projectCursor = 0
				}
				if m.projectCursor != prev {
					m.shiftGhost(prev)
					m.tagPopReveal = 0
					m.velocity -= float64(steps) * 0.4
					m.startDecrypt()
					m.followProjectCursor()
				}
			} else if m.currentView > ViewHome {
				m.scrollBy(-m.consumeNum(1))
			}
			return m, nil

		case "down", "j":
			if m.currentView == ViewProjects {
				steps := m.consumeNum(1)
				prev := m.projectCursor
				m.projectCursor += steps
				if n := views.ProjectCount(); m.projectCursor >= n {
					m.projectCursor = n - 1
				}
				if m.projectCursor < 0 {
					m.projectCursor = 0
				}
				if m.projectCursor != prev {
					m.shiftGhost(prev)
					m.tagPopReveal = 0
					m.velocity += float64(steps) * 0.4
					m.startDecrypt()
					m.followProjectCursor()
				}
			} else if m.currentView > ViewHome {
				m.scrollBy(m.consumeNum(1))
			}
			return m, nil

		// Page/half-page scrolling works on every content view, including
		// Projects where j/k are bound to cursor movement.
		case "pgdown", "ctrl+d", " ":
			if m.currentView >= ViewHome {
				m.scrollBy(m.viewportHeight() / 2)
			}
			return m, nil

		case "pgup", "ctrl+u":
			if m.currentView >= ViewHome {
				m.scrollBy(-m.viewportHeight() / 2)
			}
			return m, nil

		case "home":
			if m.currentView >= ViewHome {
				m.scrollY = 0
			}
			return m, nil

		case "end":
			if m.currentView >= ViewHome {
				m.scrollBy(1 << 20)
			}
			return m, nil

		case "g":
			// gg — handled as two consecutive g presses
			if m.currentView == ViewProjects {
				if m.numBuf == "g" {
					m.numBuf = ""
					prev := m.projectCursor
					m.projectCursor = 0
					if m.projectCursor != prev { m.shiftGhost(prev); m.tagPopReveal = 0; m.startDecrypt(); m.followProjectCursor() }
				} else {
					m.numBuf = "g"
				}
			} else if m.currentView > ViewHome {
				if m.numBuf == "g" {
					m.numBuf = ""
					m.scrollY = 0
				} else {
					m.numBuf = "g"
				}
			}
			return m, nil

		case "G":
			if m.currentView == ViewProjects {
				prev := m.projectCursor
				m.projectCursor = views.ProjectCount() - 1
				if m.projectCursor < 0 { m.projectCursor = 0 }
				if m.projectCursor != prev { m.shiftGhost(prev); m.tagPopReveal = 0; m.startDecrypt(); m.followProjectCursor() }
				m.numBuf = ""
			} else if m.currentView > ViewHome {
				m.scrollBy(1 << 20)
				m.numBuf = ""
			}
			return m, nil

		case "0", "1", "2", "3", "4", "5", "6", "7", "8", "9":
			if m.currentView >= ViewProjects && m.numBuf != "g" {
				m.numBuf += msg.String()
			}
			return m, nil

		case "enter":
			if m.currentView == ViewHome {
				m.quitPending = false
				switch m.activeTab {
				case 0:
					m.startWipe(ViewProjects, 0)
				case 1:
					m.startWipe(ViewAbout, 1)
				case 2:
					m.startWipe(ViewContacts, 2)
				case 3:
					m.startWipe(ViewResume, 3)
				case 4:
					m.startWipe(ViewNow, 4)
				}
			}
			return m, nil
		}

	case tickMsg:
		m.tickCount++

		// ── Screensaver ───────────────────────────────────────────
		if m.saverActive {
			m.updateScreensaver()
			return m, tickCmd()
		}

		// ── Particle splatter transition ──────────────────────────
		// Runs in place of the wipe: the outgoing page shatters and falls.
		if m.splatterOn && m.splatter != nil {
			if !m.splatter.Step() {
				m.splatterOn = false
				m.splatter = nil
				m.commitPendingView()
			}
			return m, tickCmd()
		}

		// ── Wipe transition ─────────────────────────────────────────────
		// Must sit above the mini-game clock. That branch returns early, so
		// while a game is on screen nothing below it ever runs — which meant
		// leaving a game relied entirely on the splatter (whose branch is
		// above) to commit the pending view. Once the splatter stopped firing
		// on every transition, [esc] out of Snake became a dead end.
		// A transition in flight outranks the game clock.
		if m.wipePhase != 0 {
			step := m.height / 4
			if step < 4 { step = 4 }
			m.wipeLines += step
			if m.wipePhase == 1 && m.wipeLines >= m.height {
				m.commitPendingView()
				m.wipePhase = 2
				m.wipeLines = 0
			} else if m.wipePhase == 2 && m.wipeLines >= m.height {
				m.wipePhase = 0
				m.wipeLines = 0
			}
			return m, tickCmd()
		}

		// ── Mini-game clock ───────────────────────────────────────
		if m.currentView == ViewGames && len(m.games) > 0 && m.gameIdx < len(m.games) {
			m.games[m.gameIdx].Step()
			return m, tickCmd()
		}

		// ── Header text effect ────────────────────────────────────
		if m.headerOn && m.headerFX != nil {
			if !m.headerFX.Step() {
				m.headerOn = false
			}
		}

		// ── Matrix rain ────────────────────────────────────────────
		if m.currentView == ViewMatrix {
			// Tick matrix every 2 ticks (~10fps) for SSH efficiency
			if m.tickCount%2 == 0 {
				m.matrixCols = views.TickMatrixColumns(m.matrixCols, m.height)
			}
			// Phase 0 → 1 after 40 ticks (2s)
			if m.matrixPhase == 0 && m.tickCount >= 40 {
				m.matrixPhase = 1
			}
			// Phase 1: lock 6 random cells per tick
			if m.matrixPhase == 1 {
				allCells := m.matrixCells
				for i := 0; i < 6 && len(m.matrixPending) > 0; i++ {
					pos := m.matrixPending[0]
					m.matrixPending = m.matrixPending[1:]
					m.matrixLocked[pos] = allCells[pos]
				}
				// All locked → phase 2 (fade)
				if len(m.matrixPending) == 0 {
					m.matrixPhase = 2
					m.matrixPhaseTick = m.tickCount
				}
			}
			// Phase 2 → boot after a 500ms hold on the finished name
			if m.matrixPhase == 2 && m.tickCount-m.matrixPhaseTick >= 10 {
				m.currentView = ViewBoot
				m.bootStartTick = m.tickCount // record when boot screen starts
			}
			return m, tickCmd()
		}

		// ── Boot sequence ──────────────────────────────────────────
		if m.currentView == ViewBoot {
			// Use elapsed ticks since boot screen appeared — NOT absolute tickCount
			// (tickCount is already ~70+ when boot starts, so all schedule entries
			//  would fire instantly if we compared against raw tickCount)
			elapsedBoot := m.tickCount - m.bootStartTick
			for i, scheduledTick := range m.bootSchedule {
				if elapsedBoot >= scheduledTick && i >= m.bootVisible {
					m.bootVisible = i + 1
				}
			}
			// All lines shown + extra pause (10 ticks = 500ms) → go to alert
			lastTick := m.bootSchedule[len(m.bootSchedule)-1]
			if m.bootVisible >= len(m.bootLines) && elapsedBoot >= lastTick+10 {
				m.currentView = ViewAlert
				m.alertPhase = 0
				m.alertPhaseTick = m.tickCount
			}
			return m, tickCmd()
		}

		// ── Alert ─────────────────────────────────────────────────
		if m.currentView == ViewAlert {
			elapsed := m.tickCount - m.alertPhaseTick
			if m.alertPhase == 0 && elapsed >= 30 { // 1500ms
				m.alertPhase = 1
				m.alertPhaseTick = m.tickCount
			} else if m.alertPhase == 1 && elapsed >= 16 { // 800ms
				m.currentView = ViewHome
			}
			return m, tickCmd()
		}

		// ── Independent star twinkle ──────────────────────────────
		for i := range m.stars {
			if m.tickCount >= m.stars[i].FlipAt {
				m.stars[i].Bright = !m.stars[i].Bright
				// Next flip in 10–40 ticks (500ms–2000ms)
				m.stars[i].FlipAt = m.tickCount + 10 + rand.Intn(30)
			}
		}

		// ── CRT scanline sweep (every tick, wraps around) ─────────
		if m.currentView == ViewHome {
			m.scanlineY = (m.tickCount / 2) % (m.height + 5)
			if m.scanlineY >= m.height {
				m.scanlineY = -1 // hide during reset gap
			}
		} else {
			m.scanlineY = -1
		}

		// -- Portrait shimmer (home only) -----------------------
		if m.currentView == ViewHome {
			if m.portraitShimFrame > 0 {
				m.portraitShimFrame--
				if m.portraitShimFrame == 0 {
					m.portraitShimRow = -1
				}
			} else if m.tickCount%40 == 0 && rand.Intn(3) == 0 {
				m.portraitShimRow   = rand.Intn(views.PortraitLines())
				m.portraitShimFrame = 3
			}
		}

		// ── Ghost cursor fade ─────────────────────────────────────
		if m.ghostFade1 > 0 { m.ghostFade1-- }
		if m.ghostFade2 > 0 { m.ghostFade2-- }

		// ── Quit pending timeout (4s) ─────────────────────────────
		if m.quitPending && m.tickCount-m.quitPendingTick > 80 {
			m.quitPending = false
		}

		// ── Exit animation — play then quit ──────────────────────
		if m.exitPhase == 1 {
			if m.tickCount-m.exitTick >= 35 {
				m.quitting = true
				return m, tea.Quit
			}
			return m, tickCmd()
		}

		// ── Konami display timeout (10s) ──────────────────────────
		if m.konamiDone && m.tickCount-m.konamiTick > 200 {
			m.konamiDone = false
		}
		m.idleGlitch = false
		if m.currentView == ViewHome {
			m.idleTicks++
			// 400 ticks @ 50ms = 20s idle threshold
			if m.idleTicks >= 400 && m.tickCount%400 == 0 {
				m.idleGlitch = true
			}
			// Long idle drops into the FX screensaver.
			if m.idleTicks >= saverIdleTicks {
				m.startScreensaver()
				return m, tickCmd()
			}
		}

		// ── Home / Splash animations ───────────────────────────────


		if m.currentView == ViewHome {
			if m.revealIdx < views.BannerLines() {
				m.revealIdx++
				// Trigger glitch when banner just completed
				if m.revealIdx == views.BannerLines() && m.glitchFrames == 0 {
					m.glitchFrames = 3
					m.glitchRunes = glitchBanner()
				}
			} else {
				// Advance glitch frames every 2 ticks (100ms per frame)
				if m.glitchFrames > 0 && m.tickCount%2 == 0 {
					m.glitchFrames--
					if m.glitchFrames > 0 {
						m.glitchRunes = glitchBanner()
					}
				}
				tagline := views.TaglineText
				if m.taglineIdx < len([]rune(tagline)) {
					m.taglineIdx++
				} else if m.cursorLeft > 0 {
					m.cursorBlink = !m.cursorBlink
					if !m.cursorBlink {
						m.cursorLeft--
					}
				} else {
					m.taglineDone = true
					m.cursorBlink = false
				}
			}
		}

		// ── Theme flash decay ───────────────────────────────────────
		if m.themeFlash > 0 {
			m.themeFlash--
		}

		// ── Ping jitter (simulated latency display) ─────────────────
		m.pingJitter--
		if m.pingJitter <= 0 {
			delta := rand.Intn(7) - 3 // -3..+3 ms random walk
			m.pingMs += delta
			if m.pingMs < 4  { m.pingMs = 4  }
			if m.pingMs > 80 { m.pingMs = 80 }
			m.pingJitter = 6 + rand.Intn(10)
		}

		// ── Live pulse (always, every 10 ticks = 500ms) ───────────
		if m.tickCount%10 == 0 {
			m.livePulse = !m.livePulse
		}

		// ── Project cascade + tag pop + lerp + decrypt ────────────
		if m.currentView == ViewProjects {
			if m.tickCount%2 == 0 && m.projectsReveal < views.ProjectCount() {
				m.projectsReveal++
			}
			if p, ok := views.ProjectAt(m.projectCursor); ok && m.tagPopReveal < len(p.Tags) {
				m.tagPopReveal++
			}

			// Lerp highlightY toward projectCursor (smooth slide)
			target := float64(m.projectCursor)
			m.highlightY += (target - m.highlightY) * 0.25
			if abs64(m.highlightY-target) < 0.01 {
				m.highlightY = target
			}

			// Momentum decay (rubber band feel)
			if abs64(m.velocity) > 0.01 {
				m.velocity *= 0.78
			} else {
				m.velocity = 0
			}

			// Decrypt reveal — advance one char per tick
			if m.decryptIdx < len(m.decryptRunes) {
				m.decryptIdx++
			}
		}

		// ── Contacts stagger + SSH flash ──────────────────────────
		if m.currentView == ViewContacts {
			if m.tickCount%3 == 0 && m.contactsReveal < len(views.AllContacts) {
				m.contactsReveal++
			}
			if m.sshFlash > 0 && m.tickCount%4 == 0 {
				m.sshFlash--
			}
		}

		return m, tickCmd()

	// ── Commit history for the time-travel view ─────────────────
	case historyMsg:
		views.SetCommits([]views.Commit(msg))
		return m, nil

	// ── Another session posted to the guestbook ─────────────────
	case guestbookMsg:
		m.guestEntries = TheGuestbook.Entries()
		return m, waitForGuestbook(m.guestNotify)

	// ── GitHub commit fetch result ──────────────────────────────
	case commitMsg:
		m.lastCommit = string(msg)
		return m, nil

	}
	return m, nil
}

// footerHeight is the number of rows renderFooterBar occupies.
const footerHeight = 2

// showFooter reports whether the terminal is tall enough to spend two rows on
// the status bar. Below that the body gets the whole screen — drawing it
// anyway would push the frame past the terminal and scroll the display.
func (m Model) showFooter() bool {
	return m.height > footerHeight+1
}

// viewportHeight is how many rows the body may use once the footer is drawn.
func (m Model) viewportHeight() int {
	h := m.height
	if m.showFooter() {
		h -= footerHeight
	}
	if h < 1 {
		h = 1
	}
	return h
}

// frameCache memoises the rendered body of views that don't animate. Those
// screens are rebuilt on every 20fps tick otherwise — About alone is ~68 rows
// of styled string assembly — and the output is byte-identical each time.
// It's a pointer so the value-receiver View/renderBody can still fill it.
type frameCache struct {
	key  string
	body string
}

// bodyCacheKey returns a signature for the current static view, or "" when
// the view animates and must be re-rendered every frame.
func (m Model) bodyCacheKey() string {
	switch m.currentView {
	case ViewAbout, ViewResume, ViewNow, ViewNeofetch, ViewTimeline:
		// Everything that can change this view's bytes goes in the key.
		return fmt.Sprintf("%d|%dx%d|t%d|s%d|w%v|tl%d|p%d|c%d|h%d",
			m.currentView, m.width, m.height, m.themeIdx, m.scrollY, m.waveOn,
			m.timelineCursor, views.ProjectCount(), len(views.Commits()),
			time.Now().Hour()) // About's greeting is time-of-day dependent
	default:
		return ""
	}
}

// renderBody returns the current view's full, unclipped content, serving
// static views from the frame cache. View() clips the result to the viewport;
// Update() measures it to clamp scrolling.
func (m Model) renderBody(theme views.Theme) string {
	key := m.bodyCacheKey()
	if key != "" && m.cache != nil && m.cache.key == key {
		return m.cache.body
	}
	body := m.renderBodyUncached(theme)
	if key != "" && m.cache != nil {
		m.cache.key, m.cache.body = key, body
	}
	return body
}

// renderBodyUncached does the actual per-view rendering.
func (m Model) renderBodyUncached(theme views.Theme) string {
	var content string

	switch m.currentView {
	case ViewMatrix:
		return views.RenderMatrix(m.renderer, m.width, m.height, m.matrixCols, m.matrixLocked, m.matrixPhase == 2, theme)
	case ViewBoot:
		return views.RenderBoot(m.renderer, m.width, m.height, m.bootVisible, m.bootLines, theme)
	case ViewAlert:
		return views.RenderAlert(m.renderer, m.width, m.height, m.alertPhase, theme)
	case ViewHome:
		starBright := make([]bool, numStars)
		for i, s := range m.stars {
			starBright[i] = s.Bright
		}
		buildInfo := fmt.Sprintf("build %s · %s", BuildCommit, runtime.Version())
		connectedSecs := int(time.Since(m.sessionStart).Seconds())
		content = views.RenderHome(m.renderer, m.width, m.height, m.revealIdx, starBright, m.taglineIdx, m.taglineDone, m.cursorBlink, m.glitchFrames, m.glitchRunes, m.lastCommit, m.sessionID, connectedSecs, buildInfo, m.scanlineY, m.idleGlitch, m.portraitShimRow, theme)
		content += m.renderTabBar(theme)
	case ViewProjects:
		content = views.RenderProjects(m.renderer, m.width, m.height, m.projectCursor, m.projectScroll, m.projectsReveal, m.tagPopReveal, m.livePulse, m.highlightY, m.decryptIdx, m.decryptRunes, m.tickCount, m.ghostCursor1, m.ghostFade1, m.ghostCursor2, m.ghostFade2, theme)
	case ViewAbout:
		content = views.RenderAbout(m.renderer, m.width, m.height, theme)
	case ViewContacts:
		content = views.RenderContacts(m.renderer, m.width, m.height, m.contactsReveal, m.sshFlash, m.contactsCopyMode, theme)
	case ViewResume:
		content = views.RenderResume(m.renderer, m.width, m.height, theme)
	case ViewNow:
		content = views.RenderNow(m.renderer, m.width, m.height, buildDateLabel(), theme)
	case ViewGames:
		if len(m.games) > 0 && m.gameIdx < len(m.games) {
			content = "\n" + m.games[m.gameIdx].Render(m.renderer, theme)
		}
	case ViewGuestbook:
		content = views.RenderGuestbook(m.renderer, m.width, m.height,
			m.guestEntries, m.guestInput, int(m.visitorCount), m.tickCount%14 < 7, theme)
	case ViewAdmin:
		if m.isAdmin {
			content = views.RenderAdmin(m.renderer, m.width, m.height, collectAdminStats(), theme)
		} else {
			content = views.RenderAdminDenied(m.renderer, theme)
		}
	case ViewTimeline:
		content = views.RenderTimeline(m.renderer, m.width, m.height, m.timelineCursor, views.Commits(), theme)
	case ViewNeofetch:
		info := m.client
		info.SessionID = m.sessionID
		secs := int(time.Since(m.sessionStart).Seconds())
		info.Connected = fmt.Sprintf("%02d:%02d", secs/60, secs%60)
		if info.Width == 0 {
			info.Width, info.Height = m.width, m.height
		}
		content = views.RenderNeofetch(m.renderer, m.width, m.height, info, theme)
	default:
		return ""
	}

	return content
}

func (m Model) View() string {
	if m.quitting {
		return m.renderExitAnimation()
	}
	// Exit animation in progress
	if m.exitPhase == 1 {
		return m.renderExitAnimation()
	}
	// Konami easter egg overlay
	if m.konamiDone {
		return m.renderKonamiEasterEgg()
	}
	// Command palette overlay
	if m.cmdActive {
		return m.renderCmdPalette()
	}

	theme := views.Themes[m.themeIdx]

	// Screensaver takes the whole screen.
	if m.saverActive {
		return m.renderScreensaver(theme)
	}
	// Particle splatter transition — the outgoing page falling apart.
	if m.splatterOn && m.splatter != nil {
		return m.splatter.Render(m.renderer, theme)
	}

	content := m.renderBody(theme)

	// Intro screens carry no footer, but still have to fit: the boot and
	// alert screens emit a fixed number of rows regardless of terminal size.
	if m.currentView < ViewHome {
		clipped, _ := views.Clip(m.renderer, content, m.height, 0, theme)
		return clipped
	}

	// Sine-wave distortion filter ([w] toggles it).
	if m.waveOn {
		content = views.SineWave(content, m.tickCount, 3)
	}

	// Clip the body to the viewport so long views (About runs ~68 rows) stay
	// reachable on a short terminal instead of overflowing off-screen.
	viewport := m.viewportHeight()
	if m.themeFlash > 0 {
		viewport-- // the flash banner borrows a row
	}
	content, _ = views.Clip(m.renderer, content, viewport, m.scrollY, theme)

	// Apply wipe mask
	if m.wipePhase != 0 {
		lines := strings.Split(content, "\n")
		for i := range lines {
			var blank bool
			if m.wipePhase == 1 {
				blank = i < m.wipeLines
			} else {
				blank = i >= m.wipeLines
			}
			if blank {
				lines[i] = ""
			}
		}
		content = strings.Join(lines, "\n")
	}

	if m.showFooter() {
		content += m.renderFooterBar()
	}

	// Theme flash overlay — a brief bright flicker on theme switch
	if m.themeFlash > 0 {
		flashS := m.renderer.NewStyle().Foreground(lipgloss.Color(theme.Primary)).Bold(true)
		content = flashS.Render("  ✨ theme: "+theme.Name) + "\n" + content
	}

	return content
}

// followProjectCursor keeps the project list window tracking the cursor, using
// the same helper the renderer uses so the two can't disagree.
func (m *Model) followProjectCursor() {
	rows := views.ProjectListRows(m.height)
	m.projectScroll = views.ProjectListTop(m.projectCursor, m.projectScroll, rows)
}

// scrollBy moves the viewport by delta rows, clamped to the current body.
func (m *Model) scrollBy(delta int) {
	theme := views.Themes[m.themeIdx]
	max := views.MaxScroll(m.renderBody(theme), m.viewportHeight())
	m.scrollY += delta
	if m.scrollY > max {
		m.scrollY = max
	}
	if m.scrollY < 0 {
		m.scrollY = 0
	}
}

// dropLastRune removes the final rune, so backspace works on multi-byte input.
func dropLastRune(s string) string {
	r := []rune(s)
	if len(r) == 0 {
		return s
	}
	return string(r[:len(r)-1])
}

// appendGuestRune adds a character to the focused compose field, enforcing the
// same limits the store applies so the UI can't promise more than it stores.
func (m *Model) appendGuestRune(s string) {
	m.guestInput.Err = ""
	if m.guestInput.Field == 0 {
		if len([]rune(m.guestInput.Name)) < maxNameLen {
			m.guestInput.Name += s
		}
	} else {
		if len([]rune(m.guestInput.Message)) < maxMessageLen {
			m.guestInput.Message += s
		}
	}
}

// startGame opens a mini-game, seeding the registry at the current size.
func (m *Model) startGame(idx int) {
	if len(m.games) == 0 {
		m.games = views.NewAllGames(m.width, m.height)
	}
	if idx < 0 || idx >= len(m.games) {
		return
	}
	m.gameIdx = idx
	m.games[idx].Resize(m.width, m.height)
	m.games[idx].Restart()
	m.startWipe(ViewGames, m.activeTab)
}

// commitPendingView performs the actual view switch once an outbound
// transition (wipe or splatter) has finished playing.
func (m *Model) commitPendingView() {
	m.currentView = m.pendingView
	m.activeTab = m.pendingTab // keep the home tab highlight in sync
	m.scrollY = 0              // every view starts at the top
	m.numBuf = ""
	if m.pendingView == ViewProjects {
		m.projectsReveal = 0
		m.tagPopReveal = 0
	}
	if m.pendingView == ViewContacts {
		m.contactsReveal = 0
		m.sshFlash = 4
	}
	m.startHeaderFX()
}

// startHeaderFX kicks off a slot-machine style reveal on the new view's
// heading. Effects are chosen per view so navigation feels varied but stable.
func (m *Model) startHeaderFX() {
	title := ""
	switch m.currentView {
	case ViewProjects:
		title = "PROJECTS"
	case ViewAbout:
		title = "ABOUT"
	case ViewContacts:
		title = "CONTACTS"
	case ViewResume:
		title = "RESUME"
	case ViewNow:
		title = "/NOW"
	default:
		m.headerOn = false
		return
	}

	all := views.NewAllTextEffects()
	if len(all) == 0 {
		return
	}
	fx := all[int(m.currentView)%len(all)]
	fx.Start([]string{title}, len([]rune(title))+2, 1)
	m.headerFX = fx
	m.headerOn = true
	m.headerFor = m.currentView
}

// startWipe begins a wipe-out → switch → wipe-in transition.
// The project cursor is reset here rather than when the wipe completes, so
// callers like the command palette can select a specific project afterwards.
func (m *Model) startWipe(target View, tab int) {
	m.pendingView = target
	m.pendingTab = tab
	if target == ViewProjects {
		m.projectCursor = 0
		m.projectScroll = 0
		m.highlightY = 0
	}

	// The particle splatter shatters the outgoing page and lets it fall away.
	// It used to run on every transition, which turned it into noise — a
	// flourish you see nine times stops reading as one.
	//
	// So it fires exactly once per session, on the first trip out of home:
	//
	//   - it's the first transition anyone sees, which makes it read as
	//     deliberate rather than as something that happens at random
	//   - home is the only screen carrying the portrait and the banner, and a
	//     dense page is the only thing worth shattering. Sparse text pages
	//     like /now just look cheap coming apart.
	//   - going back (esc/q) stays fast and quiet — the outbound trip is the
	//     one that earns an animation
	//
	// A visitor arriving on a direct route (`ssh host projects`) skips home
	// entirely and never triggers it, which is right: someone who asked for
	// one screen shouldn't be made to sit through it.
	outbound := m.currentView == ViewHome && target != ViewHome
	if !m.splatterUsed && outbound && m.width > 4 && m.height > 4 {
		// The splatter needs the outgoing frame, so it's seeded here while
		// currentView is still the old one.
		body := m.renderBody(views.Themes[m.themeIdx])
		lines := strings.Split(body, "\n")
		if len(lines) > 1 {
			sp := views.NewSplatterTextEffect()
			sp.Start(lines, m.width, m.height)
			m.splatter = sp
			m.splatterOn = true
			m.splatterUsed = true
			m.wipePhase = 0
			return
		}
		// Too degenerate to shatter — fall through to the wipe without
		// burning the one use, so it can still land later once there's a
		// real frame to work with.
	}
	m.wipePhase = 1
	m.wipeLines = 0
}

func (m Model) renderTabBar(theme views.Theme) string {
	r := m.renderer
	activeStyle   := r.NewStyle().Bold(true).Background(theme.TabActive).Foreground(lipgloss.Color("#0A0A0A")).Padding(0, 1)
	hintStyle     := r.NewStyle().Foreground(lipgloss.Color(theme.VeryDim)).Italic(true)
	sep           := "  "

	var tabs []string
	for i, name := range tabNames {
		dist := m.activeTab - i
		if dist < 0 { dist = -dist }
		switch {
		case dist == 0:
			tabs = append(tabs, activeStyle.Render(name))
		case dist == 1:
			tabs = append(tabs, r.NewStyle().Foreground(lipgloss.Color(theme.DimMid)).Render(name))
		case dist == 2:
			tabs = append(tabs, r.NewStyle().Foreground(lipgloss.Color(theme.Dim)).Render(name))
		default:
			tabs = append(tabs, r.NewStyle().Foreground(lipgloss.Color(theme.VeryDim)).Render(name))
		}
	}

	themeName := r.NewStyle().Foreground(lipgloss.Color(theme.Primary)).Italic(true).Render("[t] "+theme.Name)
	tabBar := "\n " + joinStrings(tabs, sep) + "   " + themeName + "\n"
	tabBar += "\n " + hintStyle.Render("[← → tabs · enter open · / search · t theme · s fx · w wave · q quit]") + "\n"
	return tabBar
}

func joinStrings(strs []string, sep string) string {
	result := ""
	for i, s := range strs {
		if i > 0 {
			result += sep
		}
		result += s
	}
	return result
}

func (m Model) renderFooterBar() string {
	r := m.renderer
	theme  := views.Themes[m.themeIdx]
	barBg    := r.NewStyle().Foreground(lipgloss.Color(theme.FooterText)).Background(lipgloss.Color(theme.FooterBg))
	sidStyle := r.NewStyle().Foreground(lipgloss.Color(theme.Primary)).Background(lipgloss.Color(theme.FooterBg))
	sepStyle := r.NewStyle().Foreground(lipgloss.Color(theme.Dim)).Background(lipgloss.Color(theme.FooterBg))
	qStyle   := r.NewStyle().Foreground(lipgloss.Color(theme.VeryDim)).Background(lipgloss.Color(theme.FooterBg)).Italic(true)
	confirmS := r.NewStyle().Foreground(lipgloss.Color(theme.Warning)).Background(lipgloss.Color(theme.FooterBg)).Bold(true)

	secs := int(time.Since(m.sessionStart).Seconds())
	mins := secs / 60
	s    := secs % 60

	sid := m.sessionID
	if sid == "" { sid = "--------" }

	// Ping color: green <20ms, yellow 20-40ms, orange >40ms
	pingColor := theme.Success
	if m.pingMs > 40 { pingColor = theme.Warning }
	if m.pingMs > 60 { pingColor = "#FF5555" }
	pingS := r.NewStyle().Foreground(lipgloss.Color(pingColor)).Background(lipgloss.Color(theme.FooterBg))

	visitorStr := ""
	if m.visitorCount > 0 {
		visitorStr = fmt.Sprintf("  ·  ") + barBg.Render(fmt.Sprintf("👥 %d online", m.visitorCount))
	}

	// Quit pending hint
	quitHint := qStyle.Render("[q] quit  ")
	if m.quitPending {
		quitHint = confirmS.Render("[q] confirm quit  ")
	}

	// Breadcrumb
	crumb := m.breadcrumb()

	bar := sidStyle.Render("  "+sid) +
		sepStyle.Render("  ·  ") +
		barBg.Render(fmt.Sprintf("connected: %02d:%02d", mins, s)) +
		sepStyle.Render("  ·  ") +
		pingS.Render(fmt.Sprintf("ping: %dms", m.pingMs)) +
		visitorStr +
		sepStyle.Render("  ·  ") +
		barBg.Render(crumb) +
		sepStyle.Render("  ·  ") +
		barBg.Render(theme.Name) +
		sepStyle.Render("  ·  ") +
		quitHint

	return "\n" + bar + "\n"
}

// commitMsg carries the last GitHub commit line back to the model
type commitMsg string

// fetchCommit fetches the latest commit from the GitHub API asynchronously
func fetchCommit() tea.Cmd {
	return func() tea.Msg {
		type ghCommit struct {
			Commit struct {
				Message string `json:"message"`
			} `json:"commit"`
			CommitDate string // parsed below
		}
		client := &http.Client{Timeout: 4 * time.Second}
		resp, err := client.Get("https://api.github.com/repos/Trafalgar-2006/portfolio/commits?per_page=1")
		if err != nil {
			return commitMsg("")
		}
		defer resp.Body.Close()
		var commits []struct {
			Commit struct {
				Message   string `json:"message"`
				Author struct {
					Date string `json:"date"`
				} `json:"author"`
			} `json:"commit"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&commits); err != nil || len(commits) == 0 {
			return commitMsg("")
		}
		msg := commits[0].Commit.Message
		// Trim to first line only
		if idx := strings.Index(msg, "\n"); idx != -1 {
			msg = msg[:idx]
		}
		if len(msg) > 52 {
			msg = msg[:52] + "..."
		}
		// Relative time
		pushedAt, err := time.Parse(time.RFC3339, commits[0].Commit.Author.Date)
		ago := ""
		if err == nil {
			diff := time.Since(pushedAt)
			switch {
			case diff < time.Hour:
				ago = fmt.Sprintf("%dm ago", int(diff.Minutes()))
			case diff < 24*time.Hour:
				ago = fmt.Sprintf("%dh ago", int(diff.Hours()))
			default:
				ago = fmt.Sprintf("%dd ago", int(diff.Hours()/24))
			}
		}
		return commitMsg(fmt.Sprintf("\"%s\" · %s", msg, ago))
	}
}

// glitchBanner returns a 2D slice of runes with ~20%% of non-space chars corrupted
func glitchBanner() [][]rune {
	glyphChars := []rune{'#', '@', '%', '$', '!', '?', '█', '▓', '&', '*'}
	result := make([][]rune, len(views.NameBannerLines()))
	for i, line := range views.NameBannerLines() {
		runes := []rune(line)
		for j, r := range runes {
			if r != ' ' && rand.Float32() < 0.20 {
				runes[j] = glyphChars[rand.Intn(len(glyphChars))]
			}
		}
		result[i] = runes
	}
	return result
}

// consumeNum reads m.numBuf as an integer (defaulting to def if empty), then clears it.
func (m *Model) consumeNum(def int) int {
	if m.numBuf == "" || m.numBuf == "g" {
		m.numBuf = ""
		return def
	}
	n := 0
	for _, ch := range m.numBuf {
		if ch >= '0' && ch <= '9' {
			n = n*10 + int(ch-'0')
		}
	}
	m.numBuf = ""
	if n == 0 {
		return def
	}
	return n
}

// startDecrypt seeds a fresh decrypt animation for the current project's description.
func (m *Model) startDecrypt() {
	p, ok := views.ProjectAt(m.projectCursor)
	if !ok {
		return
	}
	desc := []rune(p.Description)
	scramble := []rune("!@#$%^&*<>?/\\|~`[]{}ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789")
	runes := make([]rune, len(desc))
	for i, r := range desc {
		if r == ' ' {
			runes[i] = ' '
		} else {
			runes[i] = scramble[rand.Intn(len(scramble))]
		}
	}
	m.decryptRunes = runes
	m.decryptIdx = 0
}

// abs64 returns the absolute value of a float64.
func abs64(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}

// shiftGhost records prev cursor position into the ghost trail.
func (m *Model) shiftGhost(prev int) {
	m.ghostCursor2 = m.ghostCursor1
	m.ghostFade2   = m.ghostFade1
	m.ghostCursor1 = prev
	m.ghostFade1   = 8
}

// breadcrumb returns a home > section > detail path string for the footer.
func (m Model) breadcrumb() string {
	switch m.currentView {
	case ViewProjects:
		if p, ok := views.ProjectAt(m.projectCursor); ok {
			t := p.Title
			if len([]rune(t)) > 20 { t = string([]rune(t)[:19]) + "..." }
			return "home > projects > " + t
		}
		return "home > projects"
	case ViewAbout:
		return "home > about"
	case ViewContacts:
		return "home > contacts"
	case ViewResume:
		return "home > resume"
	case ViewNow:
		return "home > /now"
	case ViewGames:
		if len(m.games) > 0 && m.gameIdx < len(m.games) {
			return "home > games > " + m.games[m.gameIdx].Name()
		}
		return "home > games"
	case ViewNeofetch:
		return "home > system"
	case ViewGuestbook:
		return "home > guestbook"
	case ViewAdmin:
		return "home > admin"
	case ViewTimeline:
		return "home > time travel"
	default:
		return "home"
	}
}

// konamiCode is the classic cheat code sequence.
var konamiCode = []string{"up", "up", "down", "down", "left", "right", "left", "right", "b", "a"}

// trackKonami appends the key to the rolling sequence and triggers easter egg if matched.
func (m *Model) trackKonami(key string) {
	m.konamiSeq = append(m.konamiSeq, key)
	if len(m.konamiSeq) > len(konamiCode) {
		m.konamiSeq = m.konamiSeq[len(m.konamiSeq)-len(konamiCode):]
	}
	if len(m.konamiSeq) == len(konamiCode) {
		match := true
		for i, k := range konamiCode {
			if m.konamiSeq[i] != k {
				match = false
				break
			}
		}
		if match {
			m.konamiDone = true
			m.konamiTick = m.tickCount
		}
	}
}

// cmdEntry is a searchable item in the command palette.
type cmdEntry struct{ label, target string }

// allCmdEntries returns every item the command palette can navigate to.
func allCmdEntries() []cmdEntry {
	entries := []cmdEntry{
		{"Projects", "projects"},
		{"About", "about"},
		{"Contacts", "contacts"},
		{"Resume", "resume"},
		{"/now", "now"},
		{"Snake (game)", "game:0"},
		{"Tetris (game)", "game:1"},
		{"Screensaver / FX", "screensaver"},
		{"System info (neofetch)", "neofetch"},
		{"Guestbook", "guestbook"},
		{"Time travel (git history)", "timeline"},
	}
	for i, p := range views.Projects() {
		entries = append(entries, cmdEntry{p.Title, fmt.Sprintf("project:%d", i)})
	}
	return entries
}

// cmdMatches returns the palette entries matching the current query.
func (m Model) cmdMatches() []cmdEntry {
	q := strings.ToLower(strings.TrimSpace(m.cmdQuery))
	var matched []cmdEntry
	for _, e := range allCmdEntries() {
		if q == "" || strings.Contains(strings.ToLower(e.label), q) {
			matched = append(matched, e)
		}
	}
	return matched
}

// cmdMatchCount is the number of current palette matches.
func (m Model) cmdMatchCount() int { return len(m.cmdMatches()) }

// applyCmdSelection navigates to the highlighted palette entry.
func (m *Model) applyCmdSelection() {
	matched := m.cmdMatches()
	if len(matched) == 0 {
		m.cmdActive = false
		return
	}
	if m.cmdSelIdx >= len(matched) {
		m.cmdSelIdx = len(matched) - 1
	}
	target := matched[m.cmdSelIdx].target
	m.cmdActive = false
	m.cmdQuery  = ""
	m.cmdSelIdx = 0
	switch target {
	case "projects":
		m.startWipe(ViewProjects, 0)
	case "about":
		m.startWipe(ViewAbout, 1)
	case "contacts":
		m.startWipe(ViewContacts, 2)
	case "resume":
		m.startWipe(ViewResume, 3)
	case "now":
		m.startWipe(ViewNow, 4)
	case "screensaver":
		m.startScreensaver()
	case "neofetch":
		m.startWipe(ViewNeofetch, m.activeTab)
	case "guestbook":
		m.guestEntries = TheGuestbook.Entries()
		m.startWipe(ViewGuestbook, m.activeTab)
	case "timeline":
		m.timelineCursor = 0
		m.startWipe(ViewTimeline, m.activeTab)
	case "game:0":
		m.startGame(0)
	case "game:1":
		m.startGame(1)
	default:
		if strings.HasPrefix(target, "project:") {
			var idx int
			fmt.Sscanf(target, "project:%d", &idx)
			m.startWipe(ViewProjects, 0)
			m.projectCursor = idx
			m.highlightY = float64(idx)
			m.followProjectCursor()
		}
	}
}

// renderExitAnimation shows a multi-frame animated goodbye screen.
func (m Model) renderExitAnimation() string {
	r := m.renderer
	frames := []string{
		"  ...",
		"  bye.",
		"  bye..",
		"  bye...",
		"  * Thanks for visiting!",
		"  * Thanks for visiting! *",
		"  * Thanks for visiting! * -- ssh.mohith.is-a.dev",
	}
	elapsed := 0
	if m.exitTick > 0 {
		elapsed = m.tickCount - m.exitTick
	}
	frameIdx := elapsed / 5
	if frameIdx >= len(frames) {
		frameIdx = len(frames) - 1
	}
	cyanS := r.NewStyle().Foreground(lipgloss.Color("#00DFDF")).Bold(true)
	dimS  := r.NewStyle().Foreground(lipgloss.Color("#555555"))
	var b strings.Builder
	b.WriteString("\n\n")
	b.WriteString(cyanS.Render(frames[frameIdx]) + "\n\n")
	b.WriteString(dimS.Render("  github.com/trafalgar-2006/portfolio") + "\n")
	b.WriteString(dimS.Render("  ssh.mohith.is-a.dev") + "\n\n")
	return b.String()
}

// renderCmdPalette renders a fuzzy-jump command palette modal.
func (m Model) renderCmdPalette() string {
	r     := m.renderer
	theme := views.Themes[m.themeIdx]
	matched := m.cmdMatches()

	boxW := 52
	if m.width < boxW+4 {
		boxW = m.width - 4
	}
	if boxW < 24 { boxW = 24 } // guard: negative Repeat on tiny terminals
	padLeft := (m.width - boxW) / 2
	if padLeft < 0 {
		padLeft = 0
	}
	lp := strings.Repeat(" ", padLeft)

	cyanS  := r.NewStyle().Foreground(lipgloss.Color(theme.Primary))
	dimS   := r.NewStyle().Foreground(lipgloss.Color(theme.Dim))
	selS   := r.NewStyle().Background(lipgloss.Color(theme.BoxBorder)).Foreground(lipgloss.Color(theme.Primary)).Bold(true)
	boxS   := r.NewStyle().Foreground(lipgloss.Color(theme.BoxBorder))
	inputS := r.NewStyle().Foreground(lipgloss.Color(theme.Text))

	cur := "█"
	if m.tickCount%10 < 5 {
		cur = " "
	}

	var b strings.Builder
	vpad := m.height / 4
	if vpad < 2 {
		vpad = 2
	}
	b.WriteString(strings.Repeat("\n", vpad))
	b.WriteString(lp + boxS.Render("╭"+strings.Repeat("─", boxW-2)+"╮") + "\n")

	queryLine := " " + cyanS.Render("> ") + inputS.Render(m.cmdQuery) + dimS.Render(cur)
	qvis      := lipgloss.Width(queryLine)
	qpad      := boxW - 2 - qvis
	if qpad < 0 {
		qpad = 0
	}
	b.WriteString(lp + boxS.Render("│") + queryLine + strings.Repeat(" ", qpad) + boxS.Render("│") + "\n")
	b.WriteString(lp + boxS.Render("├"+strings.Repeat("─", boxW-2)+"┤") + "\n")

	maxShow := 8
	for i, e := range matched {
		if i >= maxShow {
			break
		}
		label := e.label
		if len([]rune(label)) > boxW-4 {
			label = string([]rune(label)[:boxW-5]) + "~"
		}
		inner := " " + label
		ivis  := len([]rune(inner))
		ipad  := boxW - 2 - ivis
		if ipad < 0 {
			ipad = 0
		}
		row := inner + strings.Repeat(" ", ipad)
		if i == m.cmdSelIdx {
			b.WriteString(lp + boxS.Render("│") + selS.Render(row) + boxS.Render("│") + "\n")
		} else {
			b.WriteString(lp + boxS.Render("│") + dimS.Render(row) + boxS.Render("│") + "\n")
		}
	}
	if len(matched) == 0 {
		noRes := " no results"
		npad  := boxW - 2 - len(noRes)
		if npad < 0 {
			npad = 0
		}
		b.WriteString(lp + boxS.Render("│") + dimS.Render(noRes+strings.Repeat(" ", npad)) + boxS.Render("│") + "\n")
	}
	b.WriteString(lp + boxS.Render("╰"+strings.Repeat("─", boxW-2)+"╯") + "\n")
	b.WriteString(lp + dimS.Render("  up/down  enter jump  esc cancel") + "\n")
	return b.String()
}

// renderKonamiEasterEgg shows the konami code secret screen.
func (m Model) renderKonamiEasterEgg() string {
	r     := m.renderer
	theme := views.Themes[m.themeIdx]
	cyanS    := r.NewStyle().Foreground(lipgloss.Color(theme.Primary)).Bold(true)
	goldS    := r.NewStyle().Foreground(lipgloss.Color(theme.Accent)).Bold(true)
	dimS     := r.NewStyle().Foreground(lipgloss.Color(theme.Dim))
	magentaS := r.NewStyle().Foreground(lipgloss.Color(theme.Secondary))

	elapsed := m.tickCount - m.konamiTick
	blink   := elapsed%10 < 5

	var b strings.Builder
	b.WriteString("\n\n\n")
	b.WriteString("  " + goldS.Render(">>> KONAMI CODE ACTIVATED <<<") + "\n\n")
	b.WriteString("  " + cyanS.Render("UP UP DOWN DOWN LEFT RIGHT LEFT RIGHT B A") + "\n\n")
	b.WriteString("  " + dimS.Render("You found the secret!") + "\n\n")
	b.WriteString("  " + magentaS.Render("\"Any sufficiently advanced technology") + "\n")
	b.WriteString("  " + magentaS.Render(" is indistinguishable from magic.\"") + "\n")
	b.WriteString("  " + dimS.Render("                   -- Arthur C. Clarke") + "\n\n")
	if blink {
		b.WriteString("  " + cyanS.Render("[ press any key to continue ]") + "\n")
	} else {
		b.WriteString("  " + dimS.Render("  press any key to continue  ") + "\n")
	}
	return b.String()
}


// historyMsg carries the fetched commit history for the time-travel view.
type historyMsg []views.Commit

// fetchHistory pulls recent commits from the GitHub API. Failures are silent:
// the timeline just shows its "fetching" state rather than an error screen.
func fetchHistory() tea.Cmd {
	return func() tea.Msg {
		client := &http.Client{Timeout: 8 * time.Second}
		req, err := http.NewRequest(http.MethodGet,
			"https://api.github.com/repos/Trafalgar-2006/portfolio/commits?per_page=100", nil)
		if err != nil {
			return historyMsg(nil)
		}
		req.Header.Set("Accept", "application/vnd.github+json")
		req.Header.Set("User-Agent", "ssh-portfolio")
		if tok := os.Getenv("GITHUB_TOKEN"); tok != "" {
			req.Header.Set("Authorization", "Bearer "+tok)
		}

		resp, err := client.Do(req)
		if err != nil {
			return historyMsg(nil)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return historyMsg(nil)
		}

		var payload []struct {
			SHA    string `json:"sha"`
			Commit struct {
				Message string `json:"message"`
				Author  struct {
					Name string `json:"name"`
					Date string `json:"date"`
				} `json:"author"`
			} `json:"commit"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
			return historyMsg(nil)
		}

		out := make([]views.Commit, 0, len(payload))
		for _, c := range payload {
			at, _ := time.Parse(time.RFC3339, c.Commit.Author.Date)
			out = append(out, views.Commit{
				SHA:     c.SHA,
				Message: c.Commit.Message,
				Author:  c.Commit.Author.Name,
				At:      at,
			})
		}
		return historyMsg(out)
	}
}
