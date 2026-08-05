package main

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/trafalgar-2006/ssh-portfolio/views"
)

func key(s string) tea.KeyMsg {
	switch s {
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEsc}
	case "right":
		return tea.KeyMsg{Type: tea.KeyRight}
	case "down":
		return tea.KeyMsg{Type: tea.KeyDown}
	case "up":
		return tea.KeyMsg{Type: tea.KeyUp}
	case "pgdown":
		return tea.KeyMsg{Type: tea.KeyPgDown}
	}
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

// drive applies messages and returns the resulting model.
func drive(m Model, msgs ...tea.Msg) Model {
	for _, msg := range msgs {
		nm, _ := m.Update(msg)
		m = nm.(Model)
	}
	return m
}

// settle pumps enough ticks to finish any in-flight page transition —
// either the wipe or the particle splatter.
func settle(m Model) Model {
	for i := 0; i < 200; i++ {
		nm, _ := m.Update(tickMsg{})
		m = nm.(Model)
		// Settled means: boot finished, no transition in flight, and no page
		// re-shade still streaming.
		if m.bootDone && m.wipePhase == 0 && !m.splatterOn && m.uiPhase() != stateShade {
			break
		}
	}
	return m
}

// booted returns a model past the opening sequence, sized and ready.
func booted(w, h int) Model {
	m := NewModel(nil)
	m = drive(m, tea.WindowSizeMsg{Width: w, Height: h})
	return settle(m)
}

// openAbout navigates from home to the About view.
func openAbout(t *testing.T) Model {
	t.Helper()
	m := booted(100, 36)
	m = settle(drive(m, key("right"), key("enter"))) // nav slot 1 = About
	if m.currentView != ViewAbout {
		t.Fatalf("expected ViewAbout, got %v", m.currentView)
	}
	return m
}

// The rendered frame must never exceed the terminal height, or the footer
// (and everything else) gets pushed off-screen.
func TestViewFitsTerminalHeight(t *testing.T) {
	for _, h := range []int{10, 24, 40} {
		m := NewModel(nil)
		m.currentView = ViewHome
		m = drive(m, tea.WindowSizeMsg{Width: 100, Height: h})
		for _, nav := range [][]tea.Msg{
			{key("enter")},                             // Projects
			{key("right"), key("enter")},               // About
			{key("right"), key("right"), key("enter")}, // Contacts
		} {
			mm := settle(drive(m, nav...))
			got := strings.Count(mm.View(), "\n") + 1
			if got > h {
				t.Errorf("height %d, view %v: rendered %d rows", h, mm.currentView, got)
			}
		}
	}
}

// The About page is taller than the content pane; every row must be reachable
// by scrolling, and scrolling must clamp at the end.
func TestAboutIsFullyScrollable(t *testing.T) {
	m := openAbout(t)
	theme := views.Themes[m.themeIdx]

	total := len(m.contentLines(theme))
	pane := m.contentPaneHeight()
	if total <= pane {
		t.Skipf("About fits the pane (%d rows in %d); nothing to scroll", total, pane)
	}

	// The last line of the page must become visible after scrolling.
	last := strings.TrimSpace(views.StripAnsiForTest(m.contentLines(theme)[total-1]))
	if last == "" {
		last = strings.TrimSpace(views.StripAnsiForTest(m.contentLines(theme)[total-2]))
	}
	if strings.Contains(views.StripAnsiForTest(m.View()), last) {
		t.Skip("the page already fits; nothing to prove")
	}
	for i := 0; i < total+10 && !strings.Contains(views.StripAnsiForTest(m.View()), last); i++ {
		m = drive(m, key("j"))
	}
	if !strings.Contains(views.StripAnsiForTest(m.View()), last) {
		t.Errorf("could not scroll to the last line %q (pane %d, body %d)", last, pane, total)
	}

	// Scrolling must clamp, not run away.
	m = drive(m, key("G"))
	atEnd := m.scrollY
	m = drive(m, key("j"), key("j"), key("j"))
	if m.scrollY != atEnd {
		t.Errorf("scroll ran past the end: %d -> %d", atEnd, m.scrollY)
	}
	if m.scrollY > total {
		t.Errorf("scroll offset %d exceeds the content length %d", m.scrollY, total)
	}
}

// Navigating to a new view must reset the scroll offset.
func TestScrollResetsOnNavigation(t *testing.T) {
	m := openAbout(t)
	m = drive(m, key("G"))
	if m.scrollY == 0 {
		t.Fatal("expected a non-zero scroll offset after G")
	}
	m = settle(drive(m, key("esc")))
	if m.currentView != ViewHome {
		t.Fatalf("expected ViewHome, got %v", m.currentView)
	}
	if m.scrollY != 0 {
		t.Errorf("scroll offset survived navigation: %d", m.scrollY)
	}
}

// The command palette must accept j and k as query text, not swallow them
// as navigation — "projects" contains a j.
func TestCommandPaletteAcceptsJAndK(t *testing.T) {
	m := NewModel(nil)
	m.currentView = ViewHome
	m = drive(m, tea.WindowSizeMsg{Width: 100, Height: 30}, key("/"))
	if !m.cmdActive {
		t.Fatal("palette did not open")
	}
	for _, ch := range "project" {
		m = drive(m, key(string(ch)))
	}
	if m.cmdQuery != "project" {
		t.Fatalf("query = %q, want %q", m.cmdQuery, "project")
	}
	if n := m.cmdMatchCount(); n == 0 {
		t.Error("no palette matches for \"project\"")
	}
	m = settle(drive(m, key("enter")))
	if m.currentView != ViewProjects {
		t.Errorf("palette did not navigate to Projects, got %v", m.currentView)
	}
}

// Selecting a specific project from the palette must land on that project,
// not get reset to index 0 when the wipe completes.
func TestPaletteJumpsToSpecificProject(t *testing.T) {
	if len(views.AllProjects) < 3 {
		t.Skip("need at least 3 projects")
	}
	target := views.AllProjects[2].Title

	m := NewModel(nil)
	m.currentView = ViewHome
	m = drive(m, tea.WindowSizeMsg{Width: 120, Height: 40}, key("/"))
	m.cmdQuery = target
	m.cmdSelIdx = 0
	m = settle(drive(m, key("enter")))

	if m.currentView != ViewProjects {
		t.Fatalf("expected ViewProjects, got %v", m.currentView)
	}
	if got := views.AllProjects[m.projectCursor].Title; got != target {
		t.Errorf("cursor landed on %q, want %q", got, target)
	}
}

// The palette selection index must stay within the match list.
func TestPaletteSelectionClamped(t *testing.T) {
	m := NewModel(nil)
	m.currentView = ViewHome
	m = drive(m, tea.WindowSizeMsg{Width: 100, Height: 30}, key("/"))
	for i := 0; i < 100; i++ {
		m = drive(m, key("down"))
	}
	if m.cmdSelIdx >= m.cmdMatchCount() {
		t.Errorf("selection %d out of range (%d matches)", m.cmdSelIdx, m.cmdMatchCount())
	}
}

// Resizing during the intro must rebuild the rain to the new width.
func TestMatrixRebuildsOnResize(t *testing.T) {
	m := NewModel(nil)
	if m.currentView != ViewMatrix {
		t.Fatalf("expected ViewMatrix at start, got %v", m.currentView)
	}
	m = drive(m, tea.WindowSizeMsg{Width: 200, Height: 50})
	if len(m.matrixCols) != 200 {
		t.Errorf("rain has %d columns after resize to 200", len(m.matrixCols))
	}
	for i, ln := range strings.Split(strings.TrimRight(m.View(), "\n"), "\n") {
		if w := len([]rune(views.StripAnsiForTest(ln))); w != 200 {
			t.Fatalf("rendered row %d is %d columns wide, want 200", i, w)
		}
	}
}

// The whole intro must run to the home screen without panicking at any size.
func TestIntroRunsToHome(t *testing.T) {
	for _, sz := range []tea.WindowSizeMsg{{Width: 1, Height: 1}, {Width: 80, Height: 24}, {Width: 200, Height: 60}} {
		m := drive(NewModel(nil), sz)
		for i := 0; i < 2000 && m.currentView != ViewHome; i++ {
			nm, _ := m.Update(tickMsg{})
			m = nm.(Model)
			_ = m.View()
		}
		if m.currentView != ViewHome {
			t.Errorf("size %dx%d: intro never reached home (stuck at %v)", sz.Width, sz.Height, m.currentView)
		}
	}
}

// ─────────────────── screensaver / FX integration ───────────────────

// The screensaver must open on [s], fill the screen exactly, cycle, and exit
// on an unrelated key.
func TestScreensaverLifecycle(t *testing.T) {
	const w, h = 100, 30
	m := NewModel(nil)
	m.currentView = ViewHome
	m = drive(m, tea.WindowSizeMsg{Width: w, Height: h}, key("s"))
	if !m.saverActive {
		t.Fatal("[s] did not start the screensaver")
	}

	// Every frame must exactly fill the terminal.
	for i := 0; i < 60; i++ {
		nm, _ := m.Update(tickMsg{})
		m = nm.(Model)
		lines := strings.Split(m.View(), "\n")
		if len(lines) != h {
			t.Fatalf("frame %d: %d rows, want %d", i, len(lines), h)
		}
	}

	// n advances to the next effect and keeps the saver open.
	before := m.saverIdx
	m = drive(m, key("n"))
	if !m.saverActive {
		t.Fatal("[n] closed the screensaver")
	}
	if m.saverIdx == before && len(m.saverFX) > 1 {
		t.Error("[n] did not advance the effect")
	}

	// An unrelated key exits.
	m = drive(m, key("x"))
	if m.saverActive {
		t.Error("an unrelated key did not exit the screensaver")
	}
}

// Cycling the whole registry must be safe and keep the frame sized.
func TestScreensaverCyclesAllEffects(t *testing.T) {
	const w, h = 90, 26
	m := NewModel(nil)
	m.currentView = ViewHome
	m = drive(m, tea.WindowSizeMsg{Width: w, Height: h}, key("s"))

	n := len(m.saverFX)
	if n == 0 {
		t.Fatal("no effects registered")
	}
	seen := map[string]bool{}
	for i := 0; i < n*2; i++ {
		seen[m.saverFX[m.saverIdx].Name()] = true
		for j := 0; j < 10; j++ {
			nm, _ := m.Update(tickMsg{})
			m = nm.(Model)
		}
		if lines := strings.Split(m.View(), "\n"); len(lines) != h {
			t.Fatalf("effect %q: %d rows, want %d", m.saverFX[m.saverIdx].Name(), len(lines), h)
		}
		m = drive(m, key("n"))
	}
	if len(seen) != n {
		t.Errorf("cycled through %d distinct effects, want %d", len(seen), n)
	}
}

// Mouse clicks must reach the active effect without panicking, at any coord.
func TestScreensaverMouseInteraction(t *testing.T) {
	m := NewModel(nil)
	m.currentView = ViewHome
	m = drive(m, tea.WindowSizeMsg{Width: 80, Height: 24}, key("s"))

	for _, p := range [][2]int{{0, 0}, {40, 12}, {79, 23}, {500, 500}, {-3, -3}} {
		m = drive(m, tea.MouseMsg{X: p[0], Y: p[1], Action: tea.MouseActionPress, Button: tea.MouseButtonLeft})
		nm, _ := m.Update(tickMsg{})
		m = nm.(Model)
		if lines := strings.Split(m.View(), "\n"); len(lines) != 24 {
			t.Fatalf("click at %v broke the frame: %d rows", p, len(lines))
		}
	}
}

// Navigation must be INSTANT: one keypress, one frame, you're there.
// Regression guard for the particle-splatter transition that used to sit
// between the visitor and every screen they asked for.
func TestNavigationIsInstant(t *testing.T) {
	targets := []struct {
		keys []tea.Msg
		want View
	}{
		{[]tea.Msg{key("enter")}, ViewProjects},
		{[]tea.Msg{key("right"), key("enter")}, ViewAbout},
		{[]tea.Msg{key("right"), key("right"), key("enter")}, ViewResume},
	}
	for _, tc := range targets {
		m := NewModel(nil)
		m.currentView = ViewHome
		m = drive(m, tea.WindowSizeMsg{Width: 100, Height: 30})
		m = drive(m, tc.keys...)

		// No ticks pumped: the view must already be correct.
		if m.currentView != tc.want {
			t.Errorf("navigation was not instant — landed on %v, want %v", m.currentView, tc.want)
		}
		if m.splatterOn || m.wipePhase != 0 {
			t.Errorf("a blocking transition is still in the navigation path "+
				"(splatterOn=%v wipePhase=%d)", m.splatterOn, m.wipePhase)
		}
		// And the very first frame must be sane.
		if lines := strings.Split(m.View(), "\n"); len(lines) > 30 {
			t.Fatalf("first frame overflowed: %d rows", len(lines))
		}
	}
}

// The sine-wave filter must not break the frame height.
func TestWaveToggleKeepsFrameSane(t *testing.T) {
	m := NewModel(nil)
	m.currentView = ViewHome
	m = drive(m, tea.WindowSizeMsg{Width: 100, Height: 30}, key("w"))
	if !m.waveOn {
		t.Fatal("[w] did not enable the wave filter")
	}
	for i := 0; i < 30; i++ {
		nm, _ := m.Update(tickMsg{})
		m = nm.(Model)
		if lines := strings.Split(m.View(), "\n"); len(lines) > 30 {
			t.Fatalf("wave frame %d: %d rows exceeds terminal", i, len(lines))
		}
	}
}

// Ctrl+K must open the same palette as "/".
func TestCtrlKOpensPalette(t *testing.T) {
	m := NewModel(nil)
	m.currentView = ViewHome
	m = drive(m, tea.WindowSizeMsg{Width: 100, Height: 30},
		tea.KeyMsg{Type: tea.KeyCtrlK})
	if !m.cmdActive {
		t.Error("ctrl+k did not open the command palette")
	}
}

// Launching a game from the palette must land in the game and accept input.
func TestGameLaunchesFromPalette(t *testing.T) {
	for _, tc := range []struct{ query, want string }{
		{"snake", "snake"},
		{"tetris", "tetris"},
	} {
		m := NewModel(nil)
		m.currentView = ViewHome
		m = drive(m, tea.WindowSizeMsg{Width: 100, Height: 30}, key("/"))
		m.cmdQuery = tc.query
		m = settle(drive(m, key("enter")))

		if m.currentView != ViewGames {
			t.Fatalf("%s: expected ViewGames, got %v", tc.query, m.currentView)
		}
		if got := m.games[m.gameIdx].Name(); got != tc.want {
			t.Errorf("launched %q, want %q", got, tc.want)
		}

		// Play a few frames; the frame must stay inside the terminal.
		for i := 0; i < 40; i++ {
			m = drive(m, key([]string{"left", "right", "up", "down"}[i%4]))
			nm, _ := m.Update(tickMsg{})
			m = nm.(Model)
			if n := strings.Count(m.View(), "\n") + 1; n > 30 {
				t.Fatalf("%s frame %d: %d rows exceeds terminal", tc.want, i, n)
			}
		}

		// esc leaves the game.
		m = settle(drive(m, key("esc")))
		if m.currentView != ViewHome {
			t.Errorf("%s: esc did not return home, got %v", tc.want, m.currentView)
		}
	}
}

// A client reporting a 0x0 window must not collapse the viewport to one row.
func TestZeroSizeWindowFallsBack(t *testing.T) {
	m := NewModel(nil)
	m.currentView = ViewHome
	m = drive(m, tea.WindowSizeMsg{Width: 0, Height: 0})

	if m.width != defaultWidth || m.height != defaultHeight {
		t.Errorf("0x0 window gave %dx%d, want the %dx%d fallback",
			m.width, m.height, defaultWidth, defaultHeight)
	}
	if vh := m.viewportHeight(); vh < 10 {
		t.Errorf("viewport collapsed to %d rows", vh)
	}
	// A partially-zero report must fall back on that axis only.
	m = drive(m, tea.WindowSizeMsg{Width: 120, Height: 0})
	if m.width != 120 || m.height != defaultHeight {
		t.Errorf("120x0 gave %dx%d, want 120x%d", m.width, m.height, defaultHeight)
	}
	// A real size must still be honoured.
	m = drive(m, tea.WindowSizeMsg{Width: 150, Height: 50})
	if m.width != 150 || m.height != 50 {
		t.Errorf("valid size not applied: %dx%d", m.width, m.height)
	}
}
