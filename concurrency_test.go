package main

import (
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/trafalgar-2006/ssh-portfolio/views"
)

// These tests exercise the shared state that live SSH sessions touch
// simultaneously. They are meaningful without -race (they can catch panics
// and corrupted output on their own), but their real value is under
// `CGO_ENABLED=1 go test ./... -race`, which is what CI runs.
//
// Shared mutable state reachable from more than one goroutine:
//
//	views.AllProjects  — GitHub sync worker writes; every session reads
//	views.theCommits   — EVERY session writes (per-session history fetch)
//	TheGuestbook       — any session writes; all sessions read + get notified
//	gitHubSync / waka  — background workers write; admin view reads
//	visitorCount etc.  — atomics

// simulateSession drives one Model the way a real connection would: resize,
// navigate, tick, render — repeatedly.
func simulateSession(t *testing.T, id int, stop <-chan struct{}, wg *sync.WaitGroup) {
	defer wg.Done()
	defer func() {
		if rec := recover(); rec != nil {
			t.Errorf("session %d panicked: %v", id, rec)
		}
	}()

	m := NewModel(nil)
	m = drive(m, tea.WindowSizeMsg{Width: 100 + id, Height: 30})

	// Every view a visitor can reach, including the ones added last.
	tour := []View{
		ViewHome, ViewProjects, ViewAbout, ViewContacts, ViewResume,
		ViewNow, ViewGuestbook, ViewTimeline, ViewNeofetch, ViewAdmin, ViewGames,
	}
	m.games = views.NewAllGames(m.width, m.height)

	for i := 0; ; i++ {
		select {
		case <-stop:
			return
		default:
		}

		m.currentView = tour[i%len(tour)]

		// Tick and render, which is what the runtime does 20x/second.
		nm, _ := m.Update(tickMsg{})
		m = nm.(Model)
		out := m.View()

		// The frame must stay inside the terminal even while another
		// goroutine is rewriting the project list underneath us.
		if n := strings.Count(out, "\n") + 1; n > m.height {
			t.Errorf("session %d: frame overflowed (%d rows > %d) in view %v",
				id, n, m.height, m.currentView)
			return
		}

		// Exercise the paths that index into the shared project slice.
		m = drive(m, key("j"), key("k"))
	}
}

// Many simultaneous sessions, plus the background writers, must not corrupt
// shared state or panic.
func TestManySimultaneousSessions(t *testing.T) {
	const sessions = 12

	// Restore whatever the package-level state was.
	savedProjects := views.Projects()
	savedCommits := views.Commits()
	t.Cleanup(func() {
		views.SetProjects(savedProjects)
		views.SetCommits(savedCommits)
	})

	stop := make(chan struct{})
	var wg sync.WaitGroup

	// Readers: simulated SSH connections.
	for i := 0; i < sessions; i++ {
		wg.Add(1)
		go simulateSession(t, i, stop, &wg)
	}

	// Writer 1: the GitHub sync worker replacing the project list.
	wg.Add(1)
	go func() {
		defer wg.Done()
		for n := 0; ; n++ {
			select {
			case <-stop:
				return
			default:
			}
			repos := make([]views.GitHubRepo, 0, 8)
			for j := 0; j < n%8+1; j++ {
				repos = append(repos, views.GitHubRepo{
					Name:     fmt.Sprintf("repo-%d", j),
					URL:      fmt.Sprintf("github.com/u/repo-%d", j),
					PushedAt: time.Now(),
				})
			}
			views.MergeGitHubRepos(repos)
		}
	}()

	// Writer 2: per-session commit-history fetches all writing the same global.
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func(seed int) {
			defer wg.Done()
			for n := 0; ; n++ {
				select {
				case <-stop:
					return
				default:
				}
				cs := make([]views.Commit, 0, 20)
				for j := 0; j < n%20+1; j++ {
					cs = append(cs, views.Commit{
						SHA:     fmt.Sprintf("%040d", j),
						Message: "a commit message",
						At:      time.Now().Add(-time.Duration(j) * time.Hour),
					})
				}
				views.SetCommits(cs)
			}
		}(i)
	}

	// Writer 3: guestbook traffic, which also broadcasts to subscribers.
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
			}
			TheGuestbook.Post("visitor", "hello from a concurrent session", "SESS")
			_ = TheGuestbook.Entries()
		}
	}()

	// Reader: the admin dashboard, which snapshots every worker's state.
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
			}
			_ = collectAdminStats()
		}
	}()

	time.Sleep(750 * time.Millisecond)
	close(stop)
	wg.Wait()
}

// Sessions subscribing and unsubscribing while messages are posted must not
// leak subscribers or panic on a closed channel — this mirrors visitors
// connecting and disconnecting during a busy period.
func TestSubscriberChurnUnderLoad(t *testing.T) {
	before := TheGuestbook.Subscribers()

	stop := make(chan struct{})
	var wg sync.WaitGroup

	// Churn: connect, listen briefly, disconnect.
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
				}
				ch, unsub := TheGuestbook.Subscribe()
				select {
				case <-ch:
				case <-time.After(5 * time.Millisecond):
				}
				unsub()
			}
		}()
	}

	// Constant posting throughout.
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
			}
			TheGuestbook.Post("poster", "churn test", "")
		}
	}()

	time.Sleep(500 * time.Millisecond)
	close(stop)
	wg.Wait()

	if after := TheGuestbook.Subscribers(); after != before {
		t.Errorf("subscriber leak: %d before, %d after", before, after)
	}
}

// The project list shrinking mid-session must not leave a session indexing
// past the end — the classic use-after-shrink crash.
func TestProjectListShrinkingUnderCursor(t *testing.T) {
	saved := views.Projects()
	t.Cleanup(func() { views.SetProjects(saved) })

	// Start with a long list and put the cursor near the end.
	long := make([]views.Project, 40)
	for i := range long {
		long[i] = views.Project{Title: fmt.Sprintf("Project %02d", i), Description: "d", Status: "Live"}
	}
	views.SetProjects(long)

	m := NewModel(nil)
	m = drive(m, tea.WindowSizeMsg{Width: 120, Height: 40})
	m.currentView = ViewProjects
	m.projectCursor = 39
	m.followProjectCursor()

	defer func() {
		if rec := recover(); rec != nil {
			t.Fatalf("panicked after the list shrank under the cursor: %v", rec)
		}
	}()

	// The worker replaces it with a much shorter list.
	views.SetProjects(long[:3])

	// Everything that touches the cursor must cope.
	for i := 0; i < 30; i++ {
		nm, _ := m.Update(tickMsg{})
		m = nm.(Model)
		_ = m.View()
		m = drive(m, key("j"), key("k"), key("G"))
	}

	// And all the way to empty.
	views.SetProjects(nil)
	for i := 0; i < 20; i++ {
		nm, _ := m.Update(tickMsg{})
		m = nm.(Model)
		if out := m.View(); out == "" {
			t.Error("empty project list rendered nothing at all")
		}
		m = drive(m, key("j"), key("G"), key("enter"))
	}
}
