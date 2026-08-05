package views

import (
	"fmt"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

// renderAll invokes every view at the given terminal size.
func renderAll(r *lipgloss.Renderer, w, h int) {
	th := ThemeDracula
	RenderAbout(r, w, h, th)
	RenderNow(r, w, h, "August 2026", th)
	RenderResume(r, w, h, th)
	RenderContacts(r, w, h, 99, 0, false, th)
	RenderContacts(r, w, h, 99, 2, true, th)
	RenderAlert(r, w, h, 0, th)
	RenderAlert(r, w, h, 1, th)
	RenderBoot(r, w, h, 99, mustBootLines(), th)
	RenderHome(r, w, h, 99, make([]bool, 8), 999, true, false, 0, nil, "c", "AAAA-BBBB", 61, "b", -1, false, -1, th)
	RenderProjects(r, w, h, 0, 0, 99, 99, true, 0, 0, nil, 5, -1, 0, -1, 0, th)
}

func mustBootLines() []BootLine {
	lines, _ := NewBootSequence()
	return lines
}

// Every view must survive any terminal size. Regression for the
// "strings: negative Repeat count" panics in RenderAbout / RenderNow.
func TestRenderNeverPanicsAtAnySize(t *testing.T) {
	r := lipgloss.DefaultRenderer()
	for w := 0; w <= 130; w++ {
		for _, h := range []int{0, 1, 2, 3, 5, 10, 24, 40, 120} {
			w, h := w, h
			func() {
				defer func() {
					if rec := recover(); rec != nil {
						t.Fatalf("panic at %dx%d: %v", w, h, rec)
					}
				}()
				renderAll(r, w, h)
			}()
		}
	}
}

// A content.yaml that parses to zero projects must not crash the projects view.
func TestRenderProjectsWithNoProjects(t *testing.T) {
	saved := AllProjects
	AllProjects = nil
	defer func() { AllProjects = saved }()

	defer func() {
		if rec := recover(); rec != nil {
			t.Fatalf("panic with zero projects: %v", rec)
		}
	}()
	out := RenderProjects(lipgloss.DefaultRenderer(), 120, 40, 0, 0, 0, 0, true, 0, 0, nil, 5, -1, 0, -1, 0, ThemeDracula)
	if !strings.Contains(out, "No projects") {
		t.Errorf("expected an empty-state message, got %q", out)
	}
}

// An out-of-range cursor must be clamped rather than panicking.
func TestRenderProjectsClampsCursor(t *testing.T) {
	defer func() {
		if rec := recover(); rec != nil {
			t.Fatalf("panic on out-of-range cursor: %v", rec)
		}
	}()
	for _, cursor := range []int{-5, -1, len(AllProjects), len(AllProjects) + 10} {
		RenderProjects(lipgloss.DefaultRenderer(), 120, 40, cursor, 0, 99, 99, true, 0, 0, nil, 5, -1, 0, -1, 0, ThemeDracula)
	}
}

// wrapWidth must measure display columns, not bytes. Regression for styled
// titles wrapping after ~2 words over a real SSH connection, where lipgloss
// emits escape sequences that tripled the measured length.
func TestWrapWidthMeasuresDisplayColumns(t *testing.T) {
	styled := "\x1b[1;38;2;255;215;0mEmbedGen — Domain-Specific Code LLM\x1b[0m" +
		"  \x1b[38;2;80;250;123m● Live\x1b[0m"
	if got := lipgloss.Width(styled); got != 43 {
		t.Fatalf("fixture changed: visual width %d, want 43", got)
	}
	// 43 visible columns fits in 60 — it must stay on one line.
	if lines := strings.Split(wrapWidth(styled, 60), "\n"); len(lines) != 1 {
		t.Errorf("wrapWidth split a 43-column string across %d lines at width 60", len(lines))
	}
	// And it must actually wrap when it genuinely doesn't fit.
	if lines := strings.Split(wrapWidth(styled, 20), "\n"); len(lines) < 2 {
		t.Errorf("wrapWidth failed to wrap a 43-column string at width 20")
	}
	for _, ln := range strings.Split(wrapWidth(styled, 20), "\n") {
		if w := lipgloss.Width(ln); w > 20 {
			t.Errorf("wrapped line exceeds max width: %d > 20 (%q)", w, ln)
		}
	}
}

// The About section boxes must have matching top and bottom border widths.
// Regression for measuring an ANSI-styled label with len() instead of width.
func TestAboutBoxBordersAlign(t *testing.T) {
	out := RenderAbout(lipgloss.DefaultRenderer(), 120, 40, ThemeDracula)
	var tops, bots []int
	for _, ln := range strings.Split(out, "\n") {
		v := stripAnsi(ln)
		switch {
		case strings.Contains(v, "╭"):
			tops = append(tops, lipgloss.Width(ln))
		case strings.Contains(v, "╰"):
			bots = append(bots, lipgloss.Width(ln))
		}
	}
	if len(tops) == 0 || len(tops) != len(bots) {
		t.Fatalf("expected matching box tops/bottoms, got %d tops and %d bottoms", len(tops), len(bots))
	}
	for i := range tops {
		if tops[i] != bots[i] {
			t.Errorf("box %d: top width %d != bottom width %d", i, tops[i], bots[i])
		}
	}
}

// Clip must never emit more rows than the viewport allows, and must report a
// usable maxScroll so the whole document stays reachable.
func TestClipFitsViewportAndReachesEnd(t *testing.T) {
	r := lipgloss.DefaultRenderer()
	body := RenderAbout(r, 100, 24, ThemeDracula)
	total := strings.Count(strings.TrimRight(body, "\n"), "\n") + 1

	const viewport = 22
	out, maxScroll := Clip(r, body, viewport, 0, ThemeDracula)
	if got := strings.Count(out, "\n") + 1; got > viewport {
		t.Errorf("Clip emitted %d rows for a %d-row viewport", got, viewport)
	}
	if maxScroll <= 0 {
		t.Fatalf("expected content taller than the viewport (%d rows, viewport %d)", total, viewport)
	}
	if want := MaxScroll(body, viewport); maxScroll != want {
		t.Errorf("Clip maxScroll %d != MaxScroll %d", maxScroll, want)
	}
	// Scrolling to the bottom must show the document's final line.
	out, _ = Clip(r, body, viewport, maxScroll, ThemeDracula)
	lastReal := strings.TrimRight(body, "\n")
	lastReal = lastReal[strings.LastIndex(lastReal, "\n")+1:]
	if strings.TrimSpace(stripAnsi(lastReal)) != "" && !strings.Contains(out, lastReal) {
		t.Errorf("bottom of document unreachable at maxScroll=%d", maxScroll)
	}
	// Over-scrolling is clamped, not an error.
	if _, _ = Clip(r, body, viewport, maxScroll+500, ThemeDracula); false {
		t.Fatal("unreachable")
	}
}

// Every theme must define all its color tokens — a zero value renders as an
// invisible/default color at runtime with no error.
func TestThemesFullyPopulated(t *testing.T) {
	for _, th := range Themes {
		fields := map[string]lipgloss.Color{
			"Primary": th.Primary, "Secondary": th.Secondary, "Accent": th.Accent,
			"Success": th.Success, "Warning": th.Warning, "Purple": th.Purple,
			"Text": th.Text, "DimMid": th.DimMid, "Dim": th.Dim, "VeryDim": th.VeryDim,
			"BoxBorder": th.BoxBorder, "FooterBg": th.FooterBg, "FooterText": th.FooterText,
			"TabActive": th.TabActive, "MatrixHead": th.MatrixHead, "MatrixBright": th.MatrixBright,
			"MatrixMid": th.MatrixMid, "MatrixDim": th.MatrixDim, "MatrixLocked": th.MatrixLocked,
			"StarBright": th.StarBright, "StarDim": th.StarDim, "ScanlineColor": th.ScanlineColor,
		}
		if th.Name == "" {
			t.Error("theme with empty Name")
		}
		for name, c := range fields {
			if c == "" {
				t.Errorf("theme %q: %s is unset", th.Name, name)
			}
		}
	}
}

// Matrix rain must render at any terminal height. Regression for a negative
// array index: column heads start as low as -(height+4), which overflowed the
// fixed 60-rune buffer's modulo on terminals taller than ~56 rows.
func TestMatrixRendersAtTallHeights(t *testing.T) {
	r := lipgloss.DefaultRenderer()
	for _, h := range []int{1, 24, 56, 57, 60, 80, 120, 300} {
		h := h
		func() {
			defer func() {
				if rec := recover(); rec != nil {
					t.Fatalf("panic at height %d: %v", h, rec)
				}
			}()
			cols := NewMatrixColumns(100, h)
			// Tick it forward too — heads move and columns reset over time.
			for i := 0; i < 50; i++ {
				cols = TickMatrixColumns(cols, h)
				RenderMatrix(r, 100, h, cols, map[[2]int]rune{}, i%2 == 0, ThemeDracula)
			}
		}()
	}
}

// Matrix rain must fill the full terminal width it is built for.
func TestMatrixCoversFullWidth(t *testing.T) {
	for _, w := range []int{80, 120, 200} {
		cols := NewMatrixColumns(w, 40)
		if len(cols) != w {
			t.Errorf("NewMatrixColumns(%d) produced %d columns", w, len(cols))
		}
		out := RenderMatrix(lipgloss.DefaultRenderer(), w, 10, cols, map[[2]int]rune{}, false, ThemeDracula)
		for i, ln := range strings.Split(strings.TrimRight(out, "\n"), "\n") {
			if got := lipgloss.Width(ln); got != w {
				t.Errorf("width %d row %d: rendered %d columns", w, i, got)
			}
		}
	}
}

// Every drawn box must be a perfect rectangle: each bordered row the same
// display width as its own box's top and bottom.
func TestBoxedViewsHaveAlignedBorders(t *testing.T) {
	r := lipgloss.DefaultRenderer()
	cases := map[string]string{
		"about":    RenderAbout(r, 120, 60, ThemeDracula),
		"now":      RenderNow(r, 120, 60, "August 2026", ThemeDracula),
		"projects": RenderProjects(r, 120, 60, 0, 0, 99, 99, true, 0, 0, nil, 5, -1, 0, -1, 0, ThemeDracula),
	}
	for name, out := range cases {
		var widths []int
		for _, ln := range strings.Split(out, "\n") {
			v := stripAnsi(ln)
			if strings.ContainsAny(v, "│╭╰") {
				widths = append(widths, lipgloss.Width(ln))
			}
		}
		if len(widths) < 3 {
			t.Errorf("%s: expected bordered rows, found %d", name, len(widths))
			continue
		}
		// Every bordered row in a view shares the same right edge here, since
		// each view draws a single box column.
		for i, w := range widths {
			if w != widths[0] {
				t.Errorf("%s: bordered row %d width %d != %d", name, i, w, widths[0])
				break
			}
		}
	}
}

// The contact cards must be perfect rectangles regardless of value length.
func TestContactCardsAreRectangular(t *testing.T) {
	saved := AllContacts
	AllContacts = []Contact{
		{Icon: "(@)", Label: "Email", Value: "d.mohithakshay@gmail.com"},
		{Icon: "(gh)", Label: "A Very Long Label Indeed For Testing", Value: strings.Repeat("x", 60)},
	}
	defer func() { AllContacts = saved }()

	out := RenderContacts(lipgloss.DefaultRenderer(), 120, 60, 99, 0, false, ThemeDracula)
	var widths []int
	for _, ln := range strings.Split(out, "\n") {
		v := stripAnsi(ln)
		if strings.Contains(v, "│") || strings.Contains(v, "╭") || strings.Contains(v, "╰") {
			widths = append(widths, lipgloss.Width(ln))
		}
	}
	if len(widths) == 0 {
		t.Fatal("no card rows found")
	}
	for i, w := range widths {
		if w != widths[0] {
			t.Errorf("card row %d width %d != %d (%s)", i, w, widths[0], fmt.Sprint(widths))
			break
		}
	}
}

// With more projects than fit, the list must window rather than overflow, and
// the window must always contain the cursor.
func TestProjectListWindowsAndFollowsCursor(t *testing.T) {
	saved := AllProjects
	many := make([]Project, 40)
	for i := range many {
		many[i] = Project{Title: fmt.Sprintf("Project %02d", i), Description: "d", Status: "Live"}
	}
	AllProjects = many
	defer func() { AllProjects = saved }()

	const h = 24
	rows := ProjectListRows(h)
	if rows >= len(many) {
		t.Fatalf("expected windowing: rows=%d projects=%d", rows, len(many))
	}

	// Walk the cursor the whole way down, carrying scroll like the model does.
	scroll := 0
	for cursor := 0; cursor < len(many); cursor++ {
		scroll = ProjectListTop(cursor, scroll, rows)
		if cursor < scroll || cursor >= scroll+rows {
			t.Fatalf("cursor %d outside window [%d,%d)", cursor, scroll, scroll+rows)
		}
		if scroll < 0 || scroll+rows > len(many) {
			t.Fatalf("window [%d,%d) out of bounds for %d projects", scroll, scroll+rows, len(many))
		}
		out := RenderProjects(lipgloss.DefaultRenderer(), 120, h, cursor, scroll, 99, 99, true,
			float64(cursor), 0, nil, 5, -1, 0, -1, 0, ThemeDracula)
		if n := strings.Count(out, "\n"); n > h {
			t.Fatalf("cursor %d: rendered %d rows for a %d-row terminal", cursor, n, h)
		}
		if !strings.Contains(stripAnsi(out), many[cursor].Title) {
			t.Errorf("cursor %d: selected title %q not visible", cursor, many[cursor].Title)
		}
	}
	// And back up again.
	for cursor := len(many) - 1; cursor >= 0; cursor-- {
		scroll = ProjectListTop(cursor, scroll, rows)
		if cursor < scroll || cursor >= scroll+rows {
			t.Fatalf("upward: cursor %d outside window [%d,%d)", cursor, scroll, scroll+rows)
		}
	}
}

// The timeline must render at every width. Regression for a negative slice
// bound: the message-truncation budget (inner-25) went negative on any
// terminal narrower than ~31 columns, panicking the whole session.
func TestTimelineRendersAtEveryWidth(t *testing.T) {
	cs := make([]Commit, 12)
	for i := range cs {
		cs[i] = Commit{
			SHA:     fmt.Sprintf("%040d", i),
			Message: "a reasonably long commit message that will need truncating",
		}
	}
	r := lipgloss.DefaultRenderer()
	for w := 0; w <= 140; w++ {
		for _, h := range []int{0, 1, 5, 24, 60} {
			w, h := w, h
			func() {
				defer func() {
					if rec := recover(); rec != nil {
						t.Fatalf("RenderTimeline panicked at %dx%d: %v", w, h, rec)
					}
				}()
				for _, cursor := range []int{-3, 0, 5, len(cs) - 1, len(cs) + 9} {
					RenderTimeline(r, w, h, cursor, cs, ThemeDracula)
				}
				RenderTimeline(r, w, h, 0, nil, ThemeDracula) // empty history
			}()
		}
	}
}
