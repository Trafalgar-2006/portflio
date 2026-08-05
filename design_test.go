package main

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/trafalgar-2006/ssh-portfolio/views"
)

// The braille portrait is the visual anchor. It must be present, and it must
// be the first thing on screen — not hidden behind navigation or a panel.
func TestPortraitIsTheHero(t *testing.T) {
	m := booted(120, 44)
	out := views.StripAnsiForTest(m.View())

	if !strings.ContainsAny(out, "⡀⣿⢿⣻") {
		t.Fatal("the braille portrait is not on screen")
	}
	// It must occupy the upper-left: the first content row should start with
	// portrait glyphs, not text.
	rows := strings.Split(out, "\n")
	found := -1
	for i, r := range rows {
		if strings.ContainsAny(r, "⡀⣿⢿⣻") {
			found = i
			break
		}
	}
	if found < 0 || found > 2 {
		t.Errorf("portrait first appears on row %d; it should anchor the top", found)
	}
	if lead := strings.TrimLeft(rows[found], " "); !strings.ContainsAny(string([]rune(lead)[0]), "⡀⣿⢿⣻⠄⠅⠂⠰⠉⢀⢠⢸⢕⢿⣼⣻⡞⡀") {
		t.Errorf("portrait row does not start at the left edge: %q", string([]rune(lead)[:10]))
	}
}

// The interface must be flat: exactly two horizontal rules and no box drawing.
func TestNoBoxesExactlyTwoRules(t *testing.T) {
	for _, v := range []View{ViewHome, ViewProjects, ViewAbout, ViewResume, ViewContacts, ViewNow} {
		m := booted(120, 44)
		m.currentView = v
		m = settle(m)
		out := views.StripAnsiForTest(m.View())

		for _, boxChar := range []string{"╭", "╮", "╰", "╯", "│", "├", "┤", "┌", "┐", "└", "┘"} {
			if strings.Contains(out, boxChar) {
				t.Errorf("view %v still draws box character %q", v, boxChar)
			}
		}

		rules := 0
		for _, row := range strings.Split(out, "\n") {
			trimmed := strings.TrimSpace(row)
			if len(trimmed) > 20 && strings.Trim(trimmed, "─") == "" {
				rules++
			}
		}
		if rules != 2 {
			t.Errorf("view %v has %d full-width rules, want exactly 2", v, rules)
		}
	}
}

// The boot timeline must run scanline → decode → portrait → settled, and be
// skippable at any point.
func TestBootTimeline(t *testing.T) {
	m := drive(NewModel(nil), tea.WindowSizeMsg{Width: 110, Height: 40})

	// Phase I: a blank screen with a single sweeping rule.
	first := views.StripAnsiForTest(m.View())
	if strings.ContainsAny(first, "⡀⣿") {
		t.Error("the portrait appears before the scanline phase has finished")
	}
	if !strings.Contains(first, "─") {
		t.Error("no scanline on the opening frame")
	}

	// The sweep must actually move down the screen.
	rowOf := func(s string) int {
		for i, r := range strings.Split(s, "\n") {
			if strings.Contains(r, "─") {
				return i
			}
		}
		return -1
	}
	r0 := rowOf(first)
	for i := 0; i < 3; i++ {
		nm, _ := m.Update(tickMsg{})
		m = nm.(Model)
	}
	if r1 := rowOf(views.StripAnsiForTest(m.View())); r1 <= r0 {
		t.Errorf("the scanline did not sweep downward: row %d → %d", r0, r1)
	}

	// It must settle on its own, within the specified ~900ms.
	m2 := drive(NewModel(nil), tea.WindowSizeMsg{Width: 110, Height: 40})
	ticks := 0
	for !m2.bootDone && ticks < 200 {
		nm, _ := m2.Update(tickMsg{})
		m2 = nm.(Model)
		ticks++
	}
	if !m2.bootDone {
		t.Fatal("boot never completed")
	}
	if ticks > bootPortraitEnd+2 {
		t.Errorf("boot took %d frames (~%dms), spec is ~900ms", ticks, ticks*50)
	}

	// And any key must skip it.
	m3 := drive(NewModel(nil), tea.WindowSizeMsg{Width: 110, Height: 40}, key("x"))
	if !m3.bootDone {
		t.Error("a keypress did not skip the boot sequence")
	}
}

// The decoder sizzle must resolve to the real text, never leave garbage.
func TestSizzleResolves(t *testing.T) {
	const s = "MOHITH DUGGIRALA"
	if got := views.Sizzle(s, views.SizzleDone(s, 1), 1); got != s {
		t.Errorf("sizzle did not resolve: %q", got)
	}
	// Mid-flight it must be the same length, so the layout never reflows.
	for f := 0; f < views.SizzleDone(s, 1); f++ {
		if got := views.Sizzle(s, f, 1); len([]rune(got)) != len([]rune(s)) {
			t.Fatalf("frame %d changed the string length: %q", f, got)
		}
	}
}

// Under 75 columns the header must stack instead of overlapping — and must
// never slice out of range.
func TestNarrowStacks(t *testing.T) {
	for w := 1; w < 90; w++ {
		w := w
		func() {
			defer func() {
				if rec := recover(); rec != nil {
					t.Fatalf("width %d panicked: %v", w, rec)
				}
			}()
			m := booted(w, 30)
			out := views.StripAnsiForTest(m.View())
			for i, row := range strings.Split(out, "\n") {
				if got := len([]rune(row)); got > m.width {
					t.Fatalf("width %d row %d is %d columns wide", w, i, got)
				}
			}
		}()
	}
}

// The heartbeat must live in the footer and actually cycle.
func TestHeartbeatPulses(t *testing.T) {
	seen := map[string]bool{}
	for beat := 0; beat < 14; beat++ {
		seen[views.Heartbeat(beat)] = true
	}
	if len(seen) < 3 {
		t.Errorf("the heartbeat only produced %d distinct glyphs", len(seen))
	}
	// And it must reach the rendered footer.
	m := booted(110, 40)
	rows := strings.Split(views.StripAnsiForTest(m.View()), "\n")
	last := rows[len(rows)-1]
	if !strings.ContainsAny(last, ".∘◯") {
		t.Errorf("no heartbeat in the footer: %q", last)
	}
}

// Telemetry belongs in the bottom-right, out of the content.
func TestTelemetryIsInTheFooter(t *testing.T) {
	m := booted(110, 40)
	m.visitorCount = 3
	rows := strings.Split(views.StripAnsiForTest(m.View()), "\n")
	last := rows[len(rows)-1]

	if !strings.Contains(last, "ms") || !strings.Contains(last, "online") {
		t.Errorf("telemetry missing from the footer: %q", last)
	}
	// It must sit on the right half of the line.
	if idx := strings.Index(last, "ms"); idx < len([]rune(last))/2 {
		t.Errorf("telemetry is not right-aligned (at column %d of %d)", idx, len([]rune(last)))
	}
	// And nowhere else on screen.
	for i, row := range rows[:len(rows)-1] {
		if strings.Contains(row, "online") {
			t.Errorf("telemetry leaked into the content at row %d: %q", i, row)
		}
	}
}

// Projects must use soft bullets with a blank row between entries.
func TestProjectBlocksAreSeparated(t *testing.T) {
	lines := views.BlockProjects(lipgloss.DefaultRenderer(), 110, 0, views.ThemeDracula)
	bullets := 0
	for i, ln := range lines {
		plain := views.StripAnsiForTest(ln)
		if strings.HasPrefix(strings.TrimSpace(plain), "●") ||
			strings.HasPrefix(strings.TrimSpace(plain), "◐") ||
			strings.HasPrefix(strings.TrimSpace(plain), "◇") {
			bullets++
			// The row before a bullet (other than the first) must be blank.
			if i > 0 && strings.TrimSpace(views.StripAnsiForTest(lines[i-1])) != "" {
				t.Errorf("no blank row before the entry at line %d", i)
			}
		}
	}
	if bullets == 0 {
		t.Error("no project bullets found")
	}
}
