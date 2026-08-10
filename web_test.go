package main

import (
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/trafalgar-2006/ssh-portfolio/views"
)

// readIndex returns the embedded page.
func readIndex(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile("index.html")
	if err != nil {
		t.Fatalf("index.html unreadable: %v", err)
	}
	return string(b)
}

// The web page and the SSH TUI must share one palette. If someone edits a
// theme in views/theme.go, this fails until the web catches up — which is the
// whole point of "one design system, two renderers".
func TestWebThemesMatchTerminal(t *testing.T) {
	html := readIndex(t)

	for _, th := range views.Themes {
		block := regexp.MustCompile(`(?s):root\[data-theme="` + th.Name + `"\]\s*\{(.*?)\}`).
			FindStringSubmatch(html)
		if block == nil {
			t.Errorf("theme %q is missing from the web palette", th.Name)
			continue
		}
		css := block[1]

		for _, pair := range []struct {
			cssVar string
			want   string
		}{
			{"--primary", string(th.Primary)},
			{"--secondary", string(th.Secondary)},
			{"--accent", string(th.Accent)},
			{"--success", string(th.Success)},
			{"--warning", string(th.Warning)},
			{"--purple", string(th.Purple)},
			{"--text", string(th.Text)},
		} {
			m := regexp.MustCompile(regexp.QuoteMeta(pair.cssVar) + `:\s*(#[0-9A-Fa-f]{6})`).
				FindStringSubmatch(css)
			if m == nil {
				t.Errorf("theme %q: %s not defined on the web", th.Name, pair.cssVar)
				continue
			}
			if !strings.EqualFold(m[1], pair.want) {
				t.Errorf("theme %q: web %s = %s, terminal has %s",
					th.Name, pair.cssVar, m[1], pair.want)
			}
		}
	}

	// Every theme needs a swatch, or it's unreachable in the browser.
	for _, th := range views.Themes {
		if !strings.Contains(html, `data-set="`+th.Name+`"`) {
			t.Errorf("theme %q has no swatch button", th.Name)
		}
	}
}

// The page must consume the live content endpoint rather than only its
// hardcoded fallback — that's what stops it drifting from content.yaml.
func TestWebFetchesLiveContent(t *testing.T) {
	html := readIndex(t)
	// Assert the contract, not function names: the page must fetch the live
	// endpoint and re-render the project list from what comes back.
	for _, want := range []string{"/api/content", "fetch(", "d.projects"} {
		if !strings.Contains(html, want) {
			t.Errorf("index.html is missing %q — the page would drift again", want)
		}
	}
}

// User-supplied strings reach innerHTML, so they must go through escaping.
// A guestbook name or a GitHub description could otherwise inject markup.
func TestWebEscapesInterpolatedContent(t *testing.T) {
	html := readIndex(t)
	if !strings.Contains(html, "function esc") && !strings.Contains(html, "const esc") {
		t.Fatal("no escaping helper defined")
	}
	// Every project/contact field written into a template must be wrapped.
	for _, field := range []string{"p.title", "p.desc", "p.github", "c.value", "c.label"} {
		if strings.Contains(html, "${"+field+"}") {
			t.Errorf("%s is interpolated unescaped into innerHTML", field)
		}
	}

	// Link hrefs must go through a scheme allow-list, not straight from data.
	if !strings.Contains(html, "function safeHref") {
		t.Error("contact links are built without a scheme allow-list")
	}
	for _, scheme := range []string{"javascript:", "data:", "vbscript:"} {
		if strings.Contains(strings.ToLower(html), `href="`+scheme) {
			t.Errorf("a %s href is hardcoded in the page", scheme)
		}
	}
}

// The SSH command is the point of difference; it must be prominent and
// copyable.
func TestWebSurfacesTheSSHCommand(t *testing.T) {
	html := readIndex(t)
	if strings.Count(html, "ssh mohith.is-a.dev") < 2 {
		t.Error("the SSH command should appear in the header and the callout")
	}
	if !strings.Contains(html, "copySSH") {
		t.Error("no copy-to-clipboard for the SSH command")
	}
}

// Motion must be skippable and must respect the OS reduced-motion setting.
func TestWebMotionIsConsiderate(t *testing.T) {
	html := readIndex(t)
	if !strings.Contains(html, "prefers-reduced-motion") {
		t.Error("no prefers-reduced-motion handling")
	}
	if !strings.Contains(html, "keydown") || !strings.Contains(html, "pointerdown") {
		t.Error("the intro cannot be skipped by keyboard or pointer")
	}
}

// Keyboard users need visible focus.
func TestWebHasFocusStates(t *testing.T) {
	if !strings.Contains(readIndex(t), ":focus-visible") {
		t.Error("no visible focus state for keyboard navigation")
	}
}

// The web page is a rendering of the terminal, not a generic portfolio. It
// must carry the same braille portrait and the same flat treatment.
func TestWebMirrorsTheTerminal(t *testing.T) {
	html := readIndex(t)

	// The portrait art itself must be embedded, not approximated.
	art := views.PortraitReveal(views.PortraitRows())
	if len(art) == 0 {
		t.Fatal("no portrait in the views package")
	}
	// Check a distinctive middle row survives into the page.
	row := strings.TrimSpace(art[len(art)/2])
	if row != "" && !strings.Contains(html, row) {
		t.Error("the page does not embed the terminal's braille portrait")
	}

	// Monospace throughout — this is a terminal portfolio.
	if !strings.Contains(html, "--mono") || !strings.Contains(html, "font-family:var(--mono)") {
		t.Error("the page is not set in monospace")
	}

	// Flat: no rounded cards or drop shadows dressing content up as panels.
	for _, generic := range []string{"border-radius:10px", "border-radius: 10px", "box-shadow:0 4px", "backdrop-filter"} {
		if strings.Contains(html, generic) {
			t.Errorf("the page reintroduced panel chrome: %q", generic)
		}
	}

	// The same status glyphs the terminal uses.
	for _, dot := range []string{"●", "◐", "◇"} {
		if !strings.Contains(html, dot) {
			t.Errorf("status glyph %q missing from the web page", dot)
		}
	}

	// And the same heartbeat sequence as the SSH footer.
	if !strings.Contains(html, "∘") || !strings.Contains(html, "◯") {
		t.Error("the idle heartbeat is missing from the web footer")
	}
}
