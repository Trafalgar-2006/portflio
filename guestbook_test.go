package main

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/trafalgar-2006/ssh-portfolio/views"
)

// newTestGuestbook returns an isolated guestbook backed by a temp file.
func newTestGuestbook(t *testing.T) *Guestbook {
	t.Helper()
	path := filepath.Join(t.TempDir(), "gb.jsonl")
	return &Guestbook{subs: map[int]chan struct{}{}, max: 500, path: path}
}

// Posting must store the message and hand it back.
func TestGuestbookPostAndRead(t *testing.T) {
	g := newTestGuestbook(t)
	if _, ok := g.Post("mohith", "hello from the terminal", "SESS-0001"); !ok {
		t.Fatal("valid message was rejected")
	}
	entries := g.Entries()
	if len(entries) != 1 {
		t.Fatalf("got %d entries, want 1", len(entries))
	}
	if entries[0].Name != "mohith" || entries[0].Message != "hello from the terminal" {
		t.Errorf("stored entry does not match: %+v", entries[0])
	}
	if entries[0].At.IsZero() {
		t.Error("entry has no timestamp")
	}
}

// Empty and whitespace-only messages must be rejected.
func TestGuestbookRejectsEmpty(t *testing.T) {
	g := newTestGuestbook(t)
	for _, msg := range []string{"", "   ", "\t\n", "\x00\x01"} {
		if _, ok := g.Post("someone", msg, ""); ok {
			t.Errorf("accepted an empty message %q", msg)
		}
	}
	if g.Count() != 0 {
		t.Errorf("empty messages were stored: %d entries", g.Count())
	}
}

// A missing name must fall back rather than storing a blank author.
func TestGuestbookAnonymousFallback(t *testing.T) {
	g := newTestGuestbook(t)
	g.Post("   ", "no name given", "")
	if got := g.Entries()[0].Name; got != "anonymous" {
		t.Errorf("name = %q, want %q", got, "anonymous")
	}
}

// Control characters and escape sequences must be stripped — a guestbook
// message is rendered into every other viewer's terminal.
func TestGuestbookSanitisesEscapes(t *testing.T) {
	g := newTestGuestbook(t)
	nasty := "hello \x1b[31mRED\x1b[0m \x07bell\x00null\r\nnewline"
	g.Post("attacker\x1b[2J", nasty, "")

	e := g.Entries()[0]
	if strings.ContainsAny(e.Message, "\x1b\x07\x00\r\n") {
		t.Errorf("control characters survived sanitising: %q", e.Message)
	}
	if strings.ContainsAny(e.Name, "\x1b\x07\x00") {
		t.Errorf("control characters survived in the name: %q", e.Name)
	}
	// The readable text should still be there.
	if !strings.Contains(e.Message, "hello") || !strings.Contains(e.Message, "RED") {
		t.Errorf("sanitising destroyed the readable text: %q", e.Message)
	}
}

// Over-long input must be clamped, not stored whole.
func TestGuestbookClampsLength(t *testing.T) {
	g := newTestGuestbook(t)
	g.Post(strings.Repeat("n", 500), strings.Repeat("m", 5000), "")
	e := g.Entries()[0]
	if len([]rune(e.Message)) > maxMessageLen {
		t.Errorf("message length %d exceeds %d", len([]rune(e.Message)), maxMessageLen)
	}
	if len([]rune(e.Name)) > maxNameLen {
		t.Errorf("name length %d exceeds %d", len([]rune(e.Name)), maxNameLen)
	}
}

// Messages must survive a restart.
func TestGuestbookPersistsAcrossRestart(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "gb.jsonl")

	g1 := &Guestbook{subs: map[int]chan struct{}{}, max: 500, path: path}
	g1.Post("alice", "first message", "")
	g1.Post("bob", "second message", "")

	// A fresh instance reading the same file.
	t.Setenv("GUESTBOOK_PATH", path)
	g2 := &Guestbook{subs: map[int]chan struct{}{}, max: 500}
	if err := g2.Load(); err != nil {
		t.Fatalf("reload failed: %v", err)
	}
	entries := g2.Entries()
	if len(entries) != 2 {
		t.Fatalf("reloaded %d entries, want 2", len(entries))
	}
	if entries[0].Name != "alice" || entries[1].Message != "second message" {
		t.Errorf("reloaded entries are wrong: %+v", entries)
	}
}

// A missing file on first run is normal, not an error.
func TestGuestbookLoadMissingFile(t *testing.T) {
	t.Setenv("GUESTBOOK_PATH", filepath.Join(t.TempDir(), "does-not-exist.jsonl"))
	g := &Guestbook{subs: map[int]chan struct{}{}, max: 500}
	if err := g.Load(); err != nil {
		t.Errorf("missing file reported as an error: %v", err)
	}
	if g.Count() != 0 {
		t.Error("empty guestbook has entries")
	}
}

// A corrupt line must be skipped, not lose the whole book.
func TestGuestbookSkipsCorruptLines(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "gb.jsonl")
	content := `{"name":"ok1","message":"good","at":"2026-01-01T00:00:00Z"}
this is not json at all
{"name":"ok2","message":"also good","at":"2026-01-02T00:00:00Z"}
{broken
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GUESTBOOK_PATH", path)
	g := &Guestbook{subs: map[int]chan struct{}{}, max: 500}
	if err := g.Load(); err != nil {
		t.Fatalf("load failed: %v", err)
	}
	if g.Count() != 2 {
		t.Errorf("loaded %d entries, want 2 valid ones", g.Count())
	}
}

// The retention cap must bound memory.
func TestGuestbookTrimsToMax(t *testing.T) {
	g := newTestGuestbook(t)
	g.max = 10
	for i := 0; i < 50; i++ {
		g.Post("u", "message", "")
	}
	if g.Count() != 10 {
		t.Errorf("kept %d entries, want the cap of 10", g.Count())
	}
}

// Every subscriber must be notified when someone posts.
func TestGuestbookBroadcast(t *testing.T) {
	g := newTestGuestbook(t)
	ch1, un1 := g.Subscribe()
	ch2, un2 := g.Subscribe()
	defer un1()
	defer un2()

	if got := g.Subscribers(); got != 2 {
		t.Fatalf("Subscribers() = %d, want 2", got)
	}
	g.Post("someone", "broadcast me", "")

	for i, ch := range []<-chan struct{}{ch1, ch2} {
		select {
		case <-ch:
		case <-time.After(2 * time.Second):
			t.Errorf("subscriber %d was not notified", i+1)
		}
	}
}

// A slow subscriber must not block the poster.
func TestGuestbookBroadcastNeverBlocks(t *testing.T) {
	g := newTestGuestbook(t)
	_, un := g.Subscribe() // never drained
	defer un()

	done := make(chan struct{})
	go func() {
		for i := 0; i < 100; i++ {
			g.Post("u", "m", "")
		}
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("posting blocked on an undrained subscriber")
	}
}

// Unsubscribing must remove the listener and close its channel.
func TestGuestbookUnsubscribe(t *testing.T) {
	g := newTestGuestbook(t)
	ch, unsub := g.Subscribe()
	if g.Subscribers() != 1 {
		t.Fatal("subscribe did not register")
	}
	unsub()
	if g.Subscribers() != 0 {
		t.Error("unsubscribe did not deregister")
	}
	if _, open := <-ch; open {
		t.Error("channel was not closed on unsubscribe")
	}
	unsub() // must be safe to call twice
}

// Concurrent posts and reads must not corrupt the log.
func TestGuestbookConcurrentAccess(t *testing.T) {
	g := newTestGuestbook(t)
	var wg sync.WaitGroup
	const writers, perWriter = 8, 25

	for w := 0; w < writers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < perWriter; i++ {
				g.Post("writer", "concurrent message", "")
			}
		}()
	}
	for r := 0; r < 4; r++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 100; i++ {
				for _, e := range g.Entries() {
					_ = e.Message
				}
				_ = g.Count()
			}
		}()
	}
	wg.Wait()

	if got := g.Count(); got != writers*perWriter {
		t.Errorf("stored %d entries, want %d", got, writers*perWriter)
	}
}

// The guestbook view must render at any size and show the posted messages.
func TestGuestbookRenders(t *testing.T) {
	r := lipgloss.DefaultRenderer()
	entries := []views.GuestEntry{
		{Name: "alice", Message: "great portfolio!", At: time.Now().Add(-time.Hour)},
		{Name: "bob", Message: strings.Repeat("long ", 60), At: time.Now()},
	}
	for _, sz := range [][2]int{{0, 0}, {1, 1}, {40, 10}, {80, 24}, {200, 60}} {
		for _, in := range []views.GuestbookInput{
			{},
			{Active: true, Field: 0, Name: "typing"},
			{Active: true, Field: 1, Message: "half-written", Err: "some error"},
			{JustPosted: true},
		} {
			sz, in := sz, in
			func() {
				defer func() {
					if rec := recover(); rec != nil {
						t.Fatalf("panic at %dx%d: %v", sz[0], sz[1], rec)
					}
				}()
				views.RenderGuestbook(r, sz[0], sz[1], entries, in, 3, true, views.ThemeDracula)
			}()
		}
	}
	out := views.StripAnsiForTest(views.RenderGuestbook(r, 100, 40, entries, views.GuestbookInput{}, 2, true, views.ThemeDracula))
	if !strings.Contains(out, "alice") || !strings.Contains(out, "great portfolio!") {
		t.Error("guestbook view is missing a posted message")
	}
}
