package main

import (
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/trafalgar-2006/ssh-portfolio/views"
)

// allViews is every screen a visitor can reach, including the panels added
// most recently (guestbook, admin, neofetch, timeline, games).
var allViews = []struct {
	name string
	view View
}{
	{"matrix", ViewMatrix},
	{"boot", ViewBoot},
	{"alert", ViewAlert},
	{"home", ViewHome},
	{"projects", ViewProjects},
	{"about", ViewAbout},
	{"contacts", ViewContacts},
	{"resume", ViewResume},
	{"now", ViewNow},
	{"games", ViewGames},
	{"neofetch", ViewNeofetch},
	{"guestbook", ViewGuestbook},
	{"admin", ViewAdmin},
	{"timeline", ViewTimeline},
}

// prepared returns a Model sitting on the given view with its state populated,
// so the resize path exercises real content rather than empty panels.
func prepared(v View, w, h int) Model {
	m := booted(w, h)
	m.currentView = v
	m.games = views.NewAllGames(m.width, m.height)
	m.isAdmin = true // exercise the full dashboard, not the denial screen
	m.guestEntries = []views.GuestEntry{
		{Name: "alice", Message: "a message long enough to need wrapping at narrow widths"},
		{Name: "bob", Message: "another one"},
	}
	m.client = views.ClientInfo{
		User: "guest", Client: "OpenSSH_9.9", Term: "xterm-256color",
		Width: w, Height: h, KeyType: "ssh-ed25519", MaskedIP: "203.0.x.x", Colors: 16777216,
	}
	return m
}

// Resizing while any panel is on screen must not crash, and the frame must
// always fit the new terminal.
func TestResizeOnEveryPanel(t *testing.T) {
	// Deliberately brutal: zero, one-cell, extreme aspect ratios, and the
	// jump from very large to very small (the case that catches stale
	// cached geometry).
	sizes := [][2]int{
		{0, 0}, {1, 1}, {2, 2}, {5, 3}, {20, 8}, {40, 12},
		{80, 24}, {120, 40}, {200, 60}, {300, 100},
		{10, 100}, {200, 5}, {1, 60}, {60, 1},
		{80, 24}, {0, 0}, {150, 45},
	}

	// Seed the timeline so it has content to lay out.
	saved := views.Commits()
	t.Cleanup(func() { views.SetCommits(saved) })
	cs := make([]views.Commit, 25)
	for i := range cs {
		cs[i] = views.Commit{SHA: fmt.Sprintf("%040d", i), Message: "commit message here"}
	}
	views.SetCommits(cs)

	for _, tc := range allViews {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			defer func() {
				if rec := recover(); rec != nil {
					t.Fatalf("view %s panicked during resize: %v", tc.name, rec)
				}
			}()

			m := prepared(tc.view, 100, 30)

			for _, sz := range sizes {
				m = drive(m, tea.WindowSizeMsg{Width: sz[0], Height: sz[1]})

				// Tick and render a few frames at the new size.
				for i := 0; i < 6; i++ {
					nm, _ := m.Update(tickMsg{})
					m = nm.(Model)
					out := m.View()

					// A 0x0 report falls back to the default, so measure
					// against what the model actually settled on.
					if rows := strings.Count(out, "\n") + 1; rows > m.height {
						t.Fatalf("view %s at requested %dx%d (model %dx%d): frame is %d rows",
							tc.name, sz[0], sz[1], m.width, m.height, rows)
					}
				}
			}
		})
	}
}

// Every project must stay reachable by scrolling at any terminal size — the
// flat pane replaced the windowed list, so reachability is the invariant now.
func TestEveryProjectReachableAtAnySize(t *testing.T) {
	saved := views.Projects()
	t.Cleanup(func() { views.SetProjects(saved) })

	many := make([]views.Project, 12)
	for i := range many {
		many[i] = views.Project{
			Title:       fmt.Sprintf("Project %02d", i),
			Description: "a description",
			Status:      "Live",
		}
	}
	views.SetProjects(many)

	for _, sz := range [][2]int{{120, 40}, {100, 30}, {90, 24}, {80, 20}} {
		m := booted(sz[0], sz[1])
		m.currentView = ViewProjects
		theme := views.Themes[m.themeIdx]

		last := many[len(many)-1].Title
		found := false
		for i := 0; i <= m.maxContentScroll(theme)+2 && !found; i++ {
			if strings.Contains(views.StripAnsiForTest(m.View()), last) {
				found = true
				break
			}
			m = drive(m, key("j"))
		}
		if !found {
			t.Errorf("at %dx%d the last project %q was never reachable (pane %d, body %d)",
				sz[0], sz[1], last, m.contentPaneHeight(), len(m.contentLines(theme)))
		}
	}
}

// Resizing while the screensaver is running must re-seed every effect, and
// each frame must exactly fill the new terminal.
func TestResizeDuringScreensaver(t *testing.T) {
	m := NewModel(nil)
	m.currentView = ViewHome
	m = drive(m, tea.WindowSizeMsg{Width: 100, Height: 30}, key("s"))
	if !m.saverActive {
		t.Fatal("screensaver did not start")
	}

	defer func() {
		if rec := recover(); rec != nil {
			t.Fatalf("screensaver panicked during resize: %v", rec)
		}
	}()

	// Walk the whole effect registry, resizing between each.
	for i := 0; i < len(m.saverFX)+2; i++ {
		for _, sz := range [][2]int{{200, 60}, {30, 10}, {1, 1}, {0, 0}, {120, 35}} {
			m = drive(m, tea.WindowSizeMsg{Width: sz[0], Height: sz[1]})
			// Effects are sized on entry; re-seed as the model does.
			for _, fx := range m.saverFX {
				fx.Resize(m.width, m.height)
			}
			for j := 0; j < 5; j++ {
				nm, _ := m.Update(tickMsg{})
				m = nm.(Model)
				if rows := strings.Count(m.View(), "\n") + 1; rows != m.height {
					t.Fatalf("screensaver at %dx%d: %d rows, want exactly %d",
						m.width, m.height, rows, m.height)
				}
			}
		}
		m = drive(m, key("n")) // next effect
	}
}

// Resizing mid-game must not corrupt the board or overflow the frame.
func TestResizeDuringGames(t *testing.T) {
	for idx, name := range []string{"snake", "tetris"} {
		idx, name := idx, name
		t.Run(name, func(t *testing.T) {
			defer func() {
				if rec := recover(); rec != nil {
					t.Fatalf("%s panicked during resize: %v", name, rec)
				}
			}()

			m := NewModel(nil)
			m = drive(m, tea.WindowSizeMsg{Width: 100, Height: 30})
			m.startGame(idx)
			m = settle(m)

			for _, sz := range [][2]int{{200, 60}, {40, 12}, {1, 1}, {0, 0}, {100, 30}} {
				m = drive(m, tea.WindowSizeMsg{Width: sz[0], Height: sz[1]})
				for _, g := range m.games {
					g.Resize(m.width, m.height)
				}
				for i := 0; i < 20; i++ {
					m = drive(m, key([]string{"left", "right", "up", "down", " "}[i%5]))
					nm, _ := m.Update(tickMsg{})
					m = nm.(Model)
					if rows := strings.Count(m.View(), "\n") + 1; rows > m.height {
						t.Fatalf("%s at %dx%d: frame is %d rows", name, m.width, m.height, rows)
					}
				}
			}
		})
	}
}

// Resizing while composing a guestbook message must preserve the draft — a
// visitor mid-sentence shouldn't lose what they typed to a window drag.
func TestResizeDuringGuestbookCompose(t *testing.T) {
	m := prepared(ViewGuestbook, 100, 30)
	m.guestInput.Active = true
	m.guestInput.Field = 1
	m.guestInput.Name = "mohith"
	m.guestInput.Message = "a half-written message"

	for _, sz := range [][2]int{{200, 60}, {30, 10}, {0, 0}, {100, 30}} {
		m = drive(m, tea.WindowSizeMsg{Width: sz[0], Height: sz[1]})
		nm, _ := m.Update(tickMsg{})
		m = nm.(Model)
		_ = m.View()

		if !m.guestInput.Active {
			t.Fatalf("at %dx%d the compose box closed itself", sz[0], sz[1])
		}
		if m.guestInput.Message != "a half-written message" || m.guestInput.Name != "mohith" {
			t.Fatalf("at %dx%d the draft was lost: name=%q message=%q",
				sz[0], sz[1], m.guestInput.Name, m.guestInput.Message)
		}
	}
}

// The frame cache keys on width and height, so a resize must invalidate it.
// A stale cached body would render at the old geometry after a resize.
func TestResizeInvalidatesFrameCache(t *testing.T) {
	for _, v := range []View{ViewAbout, ViewResume, ViewNow, ViewNeofetch, ViewTimeline} {
		m := prepared(v, 100, 30)
		theme := views.Themes[m.themeIdx]

		m.renderBody(theme)
		keyBefore := m.cache.key

		m = drive(m, tea.WindowSizeMsg{Width: 60, Height: 20})
		m.renderBody(theme)

		if m.cache.key == keyBefore {
			t.Errorf("view %v: resize did not invalidate the frame cache", v)
		}
	}
}
