package main

import (
	"strings"
	"testing"

	"github.com/trafalgar-2006/ssh-portfolio/views"
)

func aboutModel() Model {
	m := booted(100, 36)
	m.currentView = ViewAbout
	return m
}

// The cache key must change whenever anything that affects the rendered body
// changes. This is the real invariant — the key is what gates reuse.
func TestFrameCacheKeyInvalidates(t *testing.T) {
	mutations := []struct {
		name string
		fn   func(*Model)
	}{
		{"theme", func(m *Model) { m.themeIdx = (m.themeIdx + 1) % len(views.Themes) }},
		{"scroll", func(m *Model) { m.scrollY = 12 }},
		{"width", func(m *Model) { m.width = 70 }},
		{"height", func(m *Model) { m.height = 44 }},
		{"wave", func(m *Model) { m.waveOn = true }},
		{"view", func(m *Model) { m.currentView = ViewNow }},
		{"timeline cursor", func(m *Model) { m.currentView = ViewTimeline; m.timelineCursor = 3 }},
	}
	for _, mut := range mutations {
		m := aboutModel()
		before := m.bodyCacheKey()
		if before == "" {
			t.Fatal("About should be cacheable")
		}
		mut.fn(&m)
		if after := m.bodyCacheKey(); after == before {
			t.Errorf("%s change did not alter the cache key (%q)", mut.name, before)
		}
	}
}

// Identical state must reuse the cached body — that's the whole point.
func TestFrameCacheReuses(t *testing.T) {
	m := aboutModel()
	theme := views.Themes[m.themeIdx]

	first := m.renderBody(theme)
	if m.cache.key == "" {
		t.Fatal("cache was not populated")
	}
	cachedKey := m.cache.key

	// Poison the stored body: a second call with unchanged state must return
	// the poisoned value, proving it came from the cache rather than a rebuild.
	m.cache.body = "SENTINEL"
	if got := m.renderBody(theme); got != "SENTINEL" {
		t.Error("unchanged state re-rendered instead of using the cache")
	}
	// Changing state must bypass the poisoned entry.
	m.scrollY = 9
	if got := m.renderBody(theme); got == "SENTINEL" {
		t.Error("changed state served the stale cached body")
	}
	if m.cache.key == cachedKey {
		t.Error("cache key was not updated after the change")
	}
	_ = first
}

// Switching views must not serve another view's body.
func TestFrameCacheNotSharedAcrossViews(t *testing.T) {
	m := aboutModel()
	theme := views.Themes[m.themeIdx]

	about := views.StripAnsiForTest(m.renderBody(theme))
	m.currentView = ViewNow
	now := views.StripAnsiForTest(m.renderBody(theme))

	// The two views must render different bodies.
	if about == now {
		t.Error("the /now view was served the About body")
	}
	if !strings.Contains(about, "Experience") {
		t.Errorf("About body does not look like About: %.200s", about)
	}
}

// Animated views must never be cached — they would freeze on screen.
func TestAnimatedViewsAreNotCached(t *testing.T) {
	for _, v := range []View{ViewHome, ViewProjects, ViewMatrix, ViewBoot, ViewGames, ViewAdmin, ViewGuestbook} {
		m := NewModel(nil)
		m.currentView = v
		if key := m.bodyCacheKey(); key != "" {
			t.Errorf("view %v is cached but animates (key %q)", v, key)
		}
	}
}

// Scrolling and the wave filter act in View(); confirm they visibly change it.
func TestViewReflectsScrollAndWave(t *testing.T) {
	m := aboutModel()
	before := m.View()
	// Scroll by a step the content is guaranteed to support.
	if maxScroll := m.maxContentScroll(views.Themes[m.themeIdx]); maxScroll > 0 {
		m.scrollY = maxScroll
		if after := m.View(); after == before {
			t.Error("scrolling did not change the rendered view")
		}
	}
	m.scrollY = 0
	m.scrollY = 0
	m.waveOn = true
	if after := m.View(); after == before {
		t.Error("the wave filter did not change the rendered view")
	}
}

// A model without a cache must still render.
func TestRenderWithoutCache(t *testing.T) {
	m := aboutModel()
	m.cache = nil
	if out := m.renderBody(views.Themes[0]); out == "" {
		t.Error("rendering without a cache produced nothing")
	}
}
