package main

import (
	"bufio"
	"encoding/json"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/trafalgar-2006/ssh-portfolio/views"
)

// Guestbook: a shared message wall that every connected SSH session sees live.
//
// Storage is an append-only JSON-lines file rather than SQLite: the Docker
// image builds with CGO_ENABLED=0, and a guestbook needs append + read-all and
// nothing else, so a log file is the right size of tool. Writes are fsync'd on
// append so a container restart can't lose the last message.
//
// Config:
//
//	GUESTBOOK_PATH  file to persist to (default ./data/guestbook.jsonl)
//	GUESTBOOK_MAX   entries retained in memory (default 500)

const (
	maxMessageLen = 180
	maxNameLen    = 24
)

// Guestbook holds the shared message log and the set of live subscribers.
type Guestbook struct {
	mu      sync.RWMutex
	entries []views.GuestEntry
	path    string
	max     int

	subMu sync.Mutex
	subs  map[int]chan struct{}
	nextID int
}

// TheGuestbook is the process-wide instance.
var TheGuestbook = &Guestbook{
	subs: map[int]chan struct{}{},
	max:  500,
}

// LoadGuestbook restores the message log from disk. A missing file is normal
// on first run and is not an error.
func (g *Guestbook) Load() error {
	g.path = envOr("GUESTBOOK_PATH", "data/guestbook.jsonl")
	if v := os.Getenv("GUESTBOOK_MAX"); v != "" {
		var n int
		if _, err := fmtSscan(v, &n); err == nil && n > 0 {
			g.max = n
		}
	}

	f, err := os.Open(g.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // first run
		}
		return err
	}
	defer f.Close()

	var loaded []views.GuestEntry
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		var e views.GuestEntry
		if err := json.Unmarshal([]byte(line), &e); err != nil {
			continue // skip a corrupt line rather than losing the whole book
		}
		loaded = append(loaded, e)
	}
	if err := sc.Err(); err != nil {
		return err
	}

	g.mu.Lock()
	g.entries = loaded
	g.trimLocked()
	n := len(g.entries)
	g.mu.Unlock()

	log.Printf("Guestbook: loaded %d entries from %s", n, g.path)
	return nil
}

// fmtSscan is a tiny helper so Load doesn't need the fmt import solely for
// parsing one optional integer.
func fmtSscan(s string, out *int) (int, error) {
	n := 0
	for _, r := range s {
		if r < '0' || r > '9' {
			return 0, errNotANumber
		}
		n = n*10 + int(r-'0')
	}
	*out = n
	return 1, nil
}

var errNotANumber = &strconvError{}

type strconvError struct{}

func (e *strconvError) Error() string { return "not a number" }

func (g *Guestbook) trimLocked() {
	if g.max > 0 && len(g.entries) > g.max {
		g.entries = g.entries[len(g.entries)-g.max:]
	}
}

// sanitise strips control characters and clamps length, so a message can't
// inject escape sequences into every other viewer's terminal.
func sanitise(s string, max int) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r == '\n' || r == '\r' || r == '\t':
			b.WriteRune(' ')
		case r < 0x20 || r == 0x7f:
			// drop: control characters and ANSI introducers
		default:
			b.WriteRune(r)
		}
	}
	out := strings.TrimSpace(b.String())
	if runes := []rune(out); len(runes) > max {
		out = strings.TrimSpace(string(runes[:max]))
	}
	return out
}

// Post appends a message and notifies every live session. It returns the
// stored entry, or false when the message was empty after sanitising.
func (g *Guestbook) Post(name, msg, sessionID string) (views.GuestEntry, bool) {
	name = sanitise(name, maxNameLen)
	msg = sanitise(msg, maxMessageLen)
	if msg == "" {
		return views.GuestEntry{}, false
	}
	if name == "" {
		name = "anonymous"
	}

	e := views.GuestEntry{
		Name:    name,
		Message: msg,
		At:      time.Now().UTC(),
		Session: sessionID,
	}

	g.mu.Lock()
	g.entries = append(g.entries, e)
	g.trimLocked()
	path := g.path
	g.mu.Unlock()

	if path != "" {
		if err := appendEntry(path, e); err != nil {
			log.Printf("Guestbook: could not persist entry: %v", err)
		}
	}
	g.broadcast()
	return e, true
}

// appendEntry writes one JSON line and flushes it to disk.
func appendEntry(path string, e views.GuestEntry) error {
	if dir := dirOf(path); dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()

	line, err := json.Marshal(e)
	if err != nil {
		return err
	}
	if _, err := f.Write(append(line, '\n')); err != nil {
		return err
	}
	// fsync: a container restart shouldn't lose the message someone just left.
	return f.Sync()
}

func dirOf(path string) string {
	i := strings.LastIndexAny(path, `/\`)
	if i <= 0 {
		return ""
	}
	return path[:i]
}

// Entries returns a snapshot of the message log, newest last.
func (g *Guestbook) Entries() []views.GuestEntry {
	g.mu.RLock()
	defer g.mu.RUnlock()
	out := make([]views.GuestEntry, len(g.entries))
	copy(out, g.entries)
	return out
}

// Count is the number of stored messages.
func (g *Guestbook) Count() int {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return len(g.entries)
}

// Subscribe registers for change notifications. The returned channel receives
// a value whenever a message is posted; call the cancel func to unsubscribe.
// The channel is buffered and sends are non-blocking, so a slow session can
// never stall the poster.
func (g *Guestbook) Subscribe() (<-chan struct{}, func()) {
	ch := make(chan struct{}, 1)
	g.subMu.Lock()
	id := g.nextID
	g.nextID++
	g.subs[id] = ch
	g.subMu.Unlock()

	return ch, func() {
		g.subMu.Lock()
		if c, ok := g.subs[id]; ok {
			delete(g.subs, id)
			close(c)
		}
		g.subMu.Unlock()
	}
}

func (g *Guestbook) broadcast() {
	g.subMu.Lock()
	defer g.subMu.Unlock()
	for _, ch := range g.subs {
		select {
		case ch <- struct{}{}:
		default: // subscriber already has a pending notification
		}
	}
}

// Subscribers is the number of live listeners, for the admin view.
func (g *Guestbook) Subscribers() int {
	g.subMu.Lock()
	defer g.subMu.Unlock()
	return len(g.subs)
}
