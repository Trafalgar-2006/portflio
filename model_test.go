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
		if m.wipePhase == 0 && !m.splatterOn {
			break
		}
	}
	return m
}

// openAbout navigates from home to the About view.
func openAbout(t *testing.T) Model {
	t.Helper()
	m := NewModel(nil)
	m.currentView = ViewHome
	m = drive(m, tea.WindowSizeMsg{Width: 100, Height: 24})
	m = settle(drive(m, key("right"), key("enter"))) // tab 1 = About
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
			{key("enter")},                              // Projects
			{key("right"), key("enter")},                // About
			{key("right"), key("right"), key("enter")},  // Contacts
		} {
			mm := settle(drive(m, nav...))
			got := strings.Count(mm.View(), "\n") + 1
			if got > h {
				t.Errorf("height %d, view %v: rendered %d rows", h, mm.currentView, got)
			}
		}
	}
}

// About is ~68 rows; on a 24-row terminal every row must be reachable.
func TestAboutIsFullyScrollable(t *testing.T) {
	m := openAbout(t)
	body := m.renderBody(views.Themes[m.themeIdx])
	total := strings.Count(strings.TrimRight(body, "\n"), "\n") + 1
	if total <= m.viewportHeight() {
		t.Skipf("About fits the viewport (%d rows); nothing to scroll", total)
	}

	// Bottom of the document carries the resume link — scroll until we see it.
	const marker = "Resume"
	if strings.Contains(m.View(), marker) {
		t.Fatalf("fixture invalid: %q visible without scrolling", marker)
	}
	for i := 0; i < 200 && !strings.Contains(m.View(), marker); i++ {
		m = drive(m, key("j"))
	}
	if !strings.Contains(m.View(), marker) {
		t.Errorf("could not scroll to %q in About (viewport %d, body %d rows)", marker, m.viewportHeight(), total)
	}
	// Scrolling must clamp, not run away.
	m = drive(m, key("G"))
	atEnd := m.scrollY
	m = drive(m, key("j"), key("j"), key("j"))
	if m.scrollY != atEnd {
		t.Errorf("scroll ran past the end: %d -> %d", atEnd, m.scrollY)
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

// The splatter transition must always complete and land on the target view —
// a stalled transition would freeze navigation.
func TestSplatterTransitionAlwaysLands(t *testing.T) {
	targets := []struct {
		keys []tea.Msg
		want View
	}{
		{[]tea.Msg{key("enter")}, ViewProjects},
		{[]tea.Msg{key("right"), key("enter")}, ViewAbout},
		{[]tea.Msg{key("right"), key("right"), key("enter")}, ViewContacts},
		{[]tea.Msg{key("right"), key("right"), key("right"), key("enter")}, ViewResume},
	}
	for _, tc := range targets {
		m := NewModel(nil)
		m.currentView = ViewHome
		m = drive(m, tea.WindowSizeMsg{Width: 100, Height: 30})
		m = drive(m, tc.keys...)

		// Pump until the transition finishes, bounded.
		frames := 0
		for (m.splatterOn || m.wipePhase != 0) && frames < 300 {
			nm, _ := m.Update(tickMsg{})
			m = nm.(Model)
			if lines := strings.Split(m.View(), "\n"); len(lines) > 30 {
				t.Fatalf("transition frame %d overflowed: %d rows", frames, len(lines))
			}
			frames++
		}
		if m.splatterOn || m.wipePhase != 0 {
			t.Fatalf("transition to %v never completed", tc.want)
		}
		if m.currentView != tc.want {
			t.Errorf("landed on %v, want %v", m.currentView, tc.want)
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
