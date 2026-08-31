package views

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"
	"testing"
)

// The web page is meant to be the same portfolio as the TUI, not a lookalike.
// It drifted once already: the site was rendering a different braille portrait,
// at a different width, with its own hand-typed bio and only three of the five
// tabs. These tests pin the parts that must stay identical.
//
// This lives in package views so it can read the unexported art.

func webPage(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile("../index.html")
	if err != nil {
		t.Fatalf("read index.html: %v", err)
	}
	return string(b)
}

// jsStringArray pulls `const NAME = [ "a", "b" ];` out of the page. The
// injector emits JSON-compatible string literals precisely so this can parse
// them without a JS engine.
func jsStringArray(t *testing.T, page, name string) []string {
	t.Helper()
	re := regexp.MustCompile(`const ` + name + ` = (\[[\s\S]*?\n\s*\]);`)
	m := re.FindStringSubmatch(page)
	if m == nil {
		t.Fatalf("index.html has no `const %s = [...]` — did the injector run?", name)
	}
	var out []string
	if err := json.Unmarshal([]byte(m[1]), &out); err != nil {
		t.Fatalf("%s is not a JSON-compatible array: %v", name, err)
	}
	return out
}

func TestWebPortraitMatchesTUI(t *testing.T) {
	got := jsStringArray(t, webPage(t), "PORTRAIT")

	if len(got) != len(portraitArt) {
		t.Fatalf("portrait rows: web has %d, TUI has %d", len(got), len(portraitArt))
	}
	for i := range portraitArt {
		if got[i] != portraitArt[i] {
			t.Errorf("portrait row %d differs\n TUI: %q\n web: %q", i, portraitArt[i], got[i])
		}
	}
}

func TestWebBannerMatchesTUI(t *testing.T) {
	got := jsStringArray(t, webPage(t), "BANNER")

	if len(got) != len(nameBanner) {
		t.Fatalf("banner rows: web has %d, TUI has %d", len(got), len(nameBanner))
	}
	for i := range nameBanner {
		if got[i] != nameBanner[i] {
			t.Errorf("banner row %d differs\n TUI: %q\n web: %q", i, nameBanner[i], got[i])
		}
	}
}

func TestWebTaglineMatchesTUI(t *testing.T) {
	page := webPage(t)
	re := regexp.MustCompile(`const TAGLINE = ("(?:[^"\\]|\\.)*");`)
	m := re.FindStringSubmatch(page)
	if m == nil {
		t.Fatal("index.html has no `const TAGLINE = \"...\"`")
	}
	var got string
	if err := json.Unmarshal([]byte(m[1]), &got); err != nil {
		t.Fatalf("TAGLINE is not a JSON string: %v", err)
	}
	if got != TaglineText {
		t.Errorf("tagline differs\n TUI: %q\n web: %q", TaglineText, got)
	}
}

// The [t] key cycles the same five themes on both surfaces, in the same order.
func TestWebThemesMatchTUI(t *testing.T) {
	page := webPage(t)

	// Each theme is emitted on one line as `{ name: "x", primary: "#...", ... }`.
	block := regexp.MustCompile(`const THEMES = \[([\s\S]*?)\n\s*\];`).FindStringSubmatch(page)
	if block == nil {
		t.Fatal("index.html has no `const THEMES = [...]`")
	}
	lines := regexp.MustCompile(`\{[^}]*\}`).FindAllString(block[1], -1)
	if len(lines) != len(Themes) {
		t.Fatalf("theme count: web has %d, TUI has %d", len(lines), len(Themes))
	}

	field := func(s, key string) string {
		m := regexp.MustCompile(key + `:\s*"([^"]*)"`).FindStringSubmatch(s)
		if m == nil {
			return ""
		}
		return m[1]
	}

	for i, want := range Themes {
		got := lines[i]
		for _, c := range []struct{ key, want string }{
			{"name", want.Name},
			{"primary", string(want.Primary)},
			{"secondary", string(want.Secondary)},
			{"accent", string(want.Accent)},
			{"success", string(want.Success)},
			{"warning", string(want.Warning)},
			{"text", string(want.Text)},
			{"dim", string(want.Dim)},
			{"veryDim", string(want.VeryDim)},
			{"footerBg", string(want.FooterBg)},
			{"footerText", string(want.FooterText)},
			{"matrixLocked", string(want.MatrixLocked)},
		} {
			// `dim` would also match `dimMid`; anchor on the separator.
			if g := field(got, `\b`+c.key); g != c.want {
				t.Errorf("theme %d (%s) %s: web %q, TUI %q", i, want.Name, c.key, g, c.want)
			}
		}
	}
}

// Every tab the page can switch to needs a pane to switch to, and the footer
// needs a button for it. A typo here silently renders a blank screen.
func TestWebTabsHavePanes(t *testing.T) {
	page := webPage(t)

	m := regexp.MustCompile(`const TABS = \[([^\]]*)\]`).FindStringSubmatch(page)
	if m == nil {
		t.Fatal("index.html has no `const TABS = [...]`")
	}
	var tabs []string
	for _, raw := range strings.Split(m[1], ",") {
		if s := strings.Trim(strings.TrimSpace(raw), `'"`); s != "" {
			tabs = append(tabs, s)
		}
	}
	if len(tabs) == 0 {
		t.Fatal("TABS is empty")
	}

	// help is reachable by [?] and the palette, but isn't in the tab bar.
	for _, name := range append(tabs, "help") {
		if !strings.Contains(page, `id="pane-`+name+`"`) {
			t.Errorf("tab %q has no <div id=\"pane-%s\">", name, name)
		}
	}
	for _, name := range tabs {
		if !strings.Contains(page, `data-tab="`+name+`"`) {
			t.Errorf("tab %q has no button in the footer", name)
		}
	}

	// The TUI's five content tabs must all be present, or the web version is
	// a subset again.
	for _, want := range []string{"projects", "about", "contacts", "resume", "now"} {
		found := false
		for _, got := range tabs {
			if got == want {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("web is missing the %q tab that the TUI has", want)
		}
	}
}

// The injector must have run: an unreplaced placeholder ships a page whose
// script block is syntactically fine but has no art and no themes.
func TestWebHasNoPlaceholders(t *testing.T) {
	page := webPage(t)
	for _, ph := range []string{"__ART__", "__THEMES__"} {
		if strings.Contains(page, ph) {
			t.Errorf("index.html still contains the %s placeholder", ph)
		}
	}
}

// The page reads everything from /api/content. If it ever grows a second
// hardcoded copy of the bio or the jobs, that's the old bug coming back.
func TestWebFetchesLiveContent(t *testing.T) {
	page := webPage(t)
	if !strings.Contains(page, "/api/content") {
		t.Fatal("index.html no longer fetches /api/content")
	}
	for _, field := range []string{"experience", "education", "skills", "skillGroups", "resumeUrl"} {
		if !strings.Contains(page, field) {
			t.Errorf("page never reads %q from the payload", field)
		}
	}
}

// Guards the fallback: with no live data the page must still render, so the
// hardcoded seed has to be a well-formed shape rather than a stale copy of
// the real content.
func TestWebFallbackIsMinimal(t *testing.T) {
	page := webPage(t)
	start := strings.Index(page, "let DATA = {")
	if start < 0 {
		t.Fatal("no `let DATA = {` fallback block")
	}
	end := strings.Index(page[start:], "\n    };")
	if end < 0 {
		t.Fatal("fallback block is not terminated")
	}
	seed := page[start : start+end]

	// The seed exists to keep the page from being blank offline, not to carry
	// a second copy of the resume. Those lists must ship empty.
	for _, k := range []string{"experience", "education", "skills", "skillGroups"} {
		if !strings.Contains(seed, k+": []") {
			t.Errorf("fallback %s should be empty — content belongs in content.yaml, not the page (%s)",
				k, fmt.Sprintf("found: %t", strings.Contains(seed, k)))
		}
	}
}
