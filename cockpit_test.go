package main

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/trafalgar-2006/ssh-portfolio/views"
)

// The cockpit must fill the terminal exactly — no dead rows, no overflow.
// This is the whole point of composing onto a sized canvas.
func TestCockpitFillsTerminalExactly(t *testing.T) {
	for _, sz := range [][2]int{
		{0, 0}, {1, 1}, {20, 8}, {40, 12}, {64, 18}, {80, 24},
		{100, 30}, {120, 34}, {200, 60}, {300, 100}, {70, 60}, {200, 20},
	} {
		w, h := sz[0], sz[1]
		m := NewModel(nil)
		m.introDone, m.revealPct = true, 100
		m.currentView = ViewHome
		m = drive(m, tea.WindowSizeMsg{Width: w, Height: h})

		func() {
			defer func() {
				if rec := recover(); rec != nil {
					t.Fatalf("cockpit panicked at %dx%d: %v", w, h, rec)
				}
			}()
			for i := 0; i < 10; i++ {
				nm, _ := m.Update(tickMsg{})
				m = nm.(Model)
				out := m.View()
				lines := strings.Split(out, "\n")
				if len(lines) != m.height {
					t.Fatalf("%dx%d (model %dx%d): %d rows, want exactly %d",
						w, h, m.width, m.height, len(lines), m.height)
				}
				for y, ln := range lines {
					if got := len([]rune(views.StripAnsiForTest(ln))); got != m.width {
						t.Fatalf("%dx%d row %d: %d columns, want exactly %d",
							w, h, y, got, m.width)
					}
				}
			}
		}()
	}
}

// The rail, the work list and the status bar must all actually appear.
func TestCockpitShowsItsPanels(t *testing.T) {
	m := NewModel(nil)
	m.introDone, m.revealPct = true, 100
	m.currentView = ViewHome
	m = drive(m, tea.WindowSizeMsg{Width: 120, Height: 34})
	nm, _ := m.Update(tickMsg{})
	m = nm.(Model)

	out := views.StripAnsiForTest(m.View())
	for _, want := range []string{"MOHITH", "NAVIGATE", "Projects", "WHO", "NOW", "WORK", "? help"} {
		if !strings.Contains(out, want) {
			t.Errorf("cockpit is missing %q", want)
		}
	}
	// And the actual work must be listed.
	if p := views.Projects(); len(p) > 0 && !strings.Contains(out, p[0].Title[:12]) {
		t.Errorf("cockpit does not list the first project")
	}
}

// The entrance must never block: content is reachable from the first frame,
// and any key jumps straight to settled.
func TestIntroNeverBlocks(t *testing.T) {
	m := drive(NewModel(nil), tea.WindowSizeMsg{Width: 110, Height: 32})

	// A keypress at any point during the intro settles it immediately.
	m = drive(m, key("x"))
	if !m.introDone || m.revealPct != 100 {
		t.Errorf("a keypress did not skip the intro (done=%v reveal=%d)", m.introDone, m.revealPct)
	}
	if m.currentView != ViewHome {
		t.Errorf("skipping the intro left the view at %v", m.currentView)
	}

	// And left alone, it settles on its own within about two seconds.
	m2 := drive(NewModel(nil), tea.WindowSizeMsg{Width: 110, Height: 32})
	ticks := 0
	for !m2.introDone && ticks < 200 {
		nm, _ := m2.Update(tickMsg{})
		m2 = nm.(Model)
		ticks++
	}
	if !m2.introDone {
		t.Fatal("the intro never settled")
	}
	if ticks > introRainTicks+revealTicks+10 {
		t.Errorf("intro took %d ticks (~%.1fs), expected about %d",
			ticks, float64(ticks)*0.05, introRainTicks+revealTicks)
	}
}

// ? must open the keymap from anywhere, and any key must close it.
func TestHelpOverlayFromAnywhere(t *testing.T) {
	for _, v := range []View{ViewHome, ViewProjects, ViewAbout, ViewGuestbook, ViewNow} {
		m := NewModel(nil)
		m.introDone, m.revealPct = true, 100
		m.currentView = v
		m = drive(m, tea.WindowSizeMsg{Width: 100, Height: 30}, key("?"))

		if !m.helpOpen {
			t.Fatalf("? did not open help from %v", v)
		}
		out := views.StripAnsiForTest(m.View())
		if !strings.Contains(out, "EVERY KEY") {
			t.Errorf("help overlay did not render from %v", v)
		}
		// Every group heading must be present.
		for _, g := range views.HelpGroups() {
			if !strings.Contains(out, strings.ToUpper(g.Title)) {
				t.Errorf("help is missing the %q group", g.Title)
			}
		}
		m = drive(m, key("z"))
		if m.helpOpen {
			t.Errorf("a keypress did not close help (from %v)", v)
		}
	}
}

// Help must advertise the bindings that are genuinely undiscoverable.
func TestHelpAdvertisesHiddenFeatures(t *testing.T) {
	var all strings.Builder
	for _, g := range views.HelpGroups() {
		for _, b := range g.Bindings {
			all.WriteString(b.Keys + " " + b.What + "\n")
		}
	}
	text := all.String()
	for _, must := range []string{"theme", "effects", "guestbook", "search", "wave"} {
		if !strings.Contains(strings.ToLower(text), must) {
			t.Errorf("help never mentions %q — it stays undiscoverable", must)
		}
	}
}

// The ambient panel must be sized to fit its panel exactly, or be absent.
func TestAmbientFitsItsPanel(t *testing.T) {
	for _, sz := range [][2]int{{120, 40}, {100, 34}, {80, 24}, {64, 18}, {40, 12}} {
		w, h := sz[0], sz[1]
		aw, ah := views.CockpitAmbientSize(w, h)
		if aw == 0 || ah == 0 {
			continue // no ambient panel at this size, which is valid
		}
		if aw > w || ah > h {
			t.Errorf("%dx%d: ambient size %dx%d exceeds the terminal", w, h, aw, ah)
		}
		// An effect built at that size must render exactly that many rows.
		fx := views.NewPlasmaEffect(aw, ah)
		fx.Step()
		lines := strings.Split(fx.Render(lipgloss.DefaultRenderer(), views.ThemeDracula), "\n")
		if len(lines) != ah {
			t.Errorf("%dx%d: ambient effect produced %d rows, want %d", w, h, len(lines), ah)
		}
	}
}

// Every feature must be reachable by searching for its natural name. This is
// the discoverability contract: if you can think of it, the palette finds it.
func TestPaletteFindsEveryFeature(t *testing.T) {
	m := NewModel(nil)
	m.introDone, m.revealPct = true, 100
	m.currentView = ViewHome
	m = drive(m, tea.WindowSizeMsg{Width: 110, Height: 32})

	for _, query := range []string{
		"project", "about", "resume", "contact", "now", "guestbook",
		"snake", "tetris", "help", "wave", "time travel",
		"fire", "plasma", "tunnel", "sand", "torus", "rain", "life",
		"nord", "amber", "matrix", "cyberpunk", "dracula",
	} {
		m.cmdQuery = query
		if n := m.cmdMatchCount(); n == 0 {
			t.Errorf("nothing in the palette matches %q — that feature is undiscoverable", query)
		}
	}
}

// Choosing a theme from the palette must actually apply it.
func TestPaletteAppliesTheme(t *testing.T) {
	m := NewModel(nil)
	m.introDone, m.revealPct = true, 100
	m.currentView = ViewHome
	m = drive(m, tea.WindowSizeMsg{Width: 110, Height: 32}, key("/"))
	m.cmdQuery = "Theme: nord"
	m.cmdSelIdx = 0
	m = settle(drive(m, key("enter")))

	if got := views.Themes[m.themeIdx].Name; got != "nord" {
		t.Errorf("palette selected theme %q, want nord", got)
	}
}

// Choosing a specific effect must open the playground on that effect.
func TestPaletteOpensSpecificEffect(t *testing.T) {
	m := NewModel(nil)
	m.introDone, m.revealPct = true, 100
	m.currentView = ViewHome
	m = drive(m, tea.WindowSizeMsg{Width: 110, Height: 32}, key("/"))
	m.cmdQuery = "Effect: fire"
	m.cmdSelIdx = 0
	m = drive(m, key("enter"))

	if !m.saverActive {
		t.Fatal("selecting an effect did not open the playground")
	}
	if got := m.saverFX[m.saverIdx].Name(); got != "fire" {
		t.Errorf("playground opened on %q, want fire", got)
	}
}
