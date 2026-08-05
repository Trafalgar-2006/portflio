package main

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/ssh"
	"github.com/charmbracelet/wish"
	"github.com/charmbracelet/wish/activeterm"
	"github.com/charmbracelet/wish/bubbletea"
	"github.com/charmbracelet/wish/logging"
	"github.com/muesli/termenv"
	"github.com/trafalgar-2006/ssh-portfolio/config"
	"github.com/trafalgar-2006/ssh-portfolio/views"
	gossh "golang.org/x/crypto/ssh"
)

// visitorCount tracks concurrent active SSH sessions (atomic, safe for concurrent access)
var visitorCount atomic.Int64

// startedAt is the process start time, reported by /health as uptime.
var startedAt = time.Now()

//go:embed index.html
var indexHTML embed.FS

func main() {
	// Must come first: SSH_ENABLED is read a few lines below and decides the
	// whole execution path, and WAKATIME_API_KEY / GITHUB_SYNC are read once
	// at startup — anything not in place by then stays off for good.
	initEnv()

	// Load content.yaml — falls back to hardcoded data if file not found
	if err := config.Load("content.yaml"); err != nil {
		log.Printf("content.yaml not found, using hardcoded content: %v", err)
	} else {
		views.LoadFromConfig()
	}

	sshEnabled := os.Getenv("SSH_ENABLED")
	if sshEnabled == "" {
		sshEnabled = "false"
	}

	if sshEnabled == "true" {
		runSSHServer()
	} else {
		runLocalTUI()
	}
}

func runLocalTUI() {
	p := tea.NewProgram(
		NewModel(nil),
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error running TUI: %v\n", err)
		os.Exit(1)
	}
}

func runSSHServer() {
	host := os.Getenv("HOST")
	if host == "" {
		host = "0.0.0.0"
	}
	port := os.Getenv("SSH_PORT")
	if port == "" {
		port = "23234"
	}

	s, err := wish.NewServer(
		wish.WithAddress(net.JoinHostPort(host, port)),
		wish.WithHostKeyPath(".ssh/id_ed25519"),
		// Accept ALL connections — public portfolio, no auth needed
		// WithPublicKeyAuth: clients that have SSH keys (Linux/Mac)
		wish.WithPublicKeyAuth(func(_ ssh.Context, _ ssh.PublicKey) bool {
			return true
		}),
		// WithKeyboardInteractiveAuth: clients with NO SSH keys (fresh Windows)
		wish.WithKeyboardInteractiveAuth(func(_ ssh.Context, _ gossh.KeyboardInteractiveChallenge) bool {
			return true
		}),
		wish.WithMiddleware(
			bubbletea.Middleware(teaHandler),
			activeterm.Middleware(),
			logging.Middleware(),
		),
	)
	if err != nil {
		log.Fatalf("Could not create SSH server: %v", err)
	}

	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	// Restore the guestbook before accepting connections.
	if err := TheGuestbook.Load(); err != nil {
		log.Printf("Guestbook: could not load history: %v", err)
	}

	// Background workers: GitHub project sync and (optional) WakaTime stats.
	syncCtx, cancelSync := context.WithCancel(context.Background())
	defer cancelSync()
	StartGitHubSync(syncCtx)
	StartWakaTime(syncCtx)

	// HTTP server: web portfolio at / and health check at /health.
	// Most PaaS platforms (Railway, Render, Fly, Cloud Run) inject the port to
	// bind as $PORT — honour it, or the health check never comes up.
	httpPort := resolveHTTPPort(os.Getenv("PORT"), port)

	go func() {
		mux := http.NewServeMux()

		// Serve the web portfolio (index.html embedded in the binary)
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			data, err := indexHTML.ReadFile("index.html")
			if err != nil {
				http.Error(w, "portfolio not found", http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Header().Set("Cache-Control", "public, max-age=3600")
			w.Write(data)
		})

		// Live content for the web portfolio, so the page renders the same
		// projects the SSH TUI does — including GitHub-synced ones — instead
		// of a hardcoded copy that drifts.
		mux.HandleFunc("/api/content", handleAPIContent)

		// Health check for UptimeRobot / Railway. Returns JSON so the monitor
		// carries useful signal (uptime, build, live sessions) rather than "OK".
		mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.Header().Set("Cache-Control", "no-store")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]any{
				"status":   "ok",
				"uptime":   time.Since(startedAt).Round(time.Second).String(),
				"uptimeMs": time.Since(startedAt).Milliseconds(),
				"sessions": visitorCount.Load(),
				"commit":   BuildCommit,
				"built":    BuildDate,
				"go":       runtime.Version(),
			})
		})

		addr := net.JoinHostPort("", httpPort)
		log.Printf("Web portfolio + health check listening on %s", addr)
		// Report loudly, but do NOT exit. This goroutine used to call
		// log.Fatalf, which killed the whole process — including a perfectly
		// healthy SSH server — and on a platform that restarts on exit that
		// turns a degraded web server into a permanent crash loop.
		// A portfolio still reachable over SSH beats one that's down entirely.
		if err := http.ListenAndServe(addr, mux); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("ERROR: web server stopped: %v", err)
			log.Printf("ERROR: the web portfolio and /health are DOWN; SSH is unaffected and still serving")
		}
	}()

	log.Printf("Starting SSH server on %s:%s", host, port)
	log.Printf("Connect with: ssh localhost -p %s", port)

	go func() {
		if err := s.ListenAndServe(); err != nil && !errors.Is(err, ssh.ErrServerClosed) {
			log.Fatalf("SSH server error: %v", err)
		}
	}()

	<-done
	log.Println("Shutting down SSH server...")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := s.Shutdown(ctx); err != nil {
		log.Fatalf("Could not shutdown server: %v", err)
	}
}

// resolveHTTPPort decides which port the web server binds, given the raw
// $PORT value and the port SSH has already claimed.
//
// The two listeners cannot share a port. Configs that predate the SSH_PORT
// split often set PORT to the SSH port (the old README told people to), and
// taking that literally makes the HTTP bind fail on every boot. Detect it here
// and step aside instead, so a stale environment variable degrades the web
// server rather than taking the whole service down.
func resolveHTTPPort(rawPort, sshPort string) string {
	httpPort := strings.TrimSpace(rawPort)
	if httpPort == "" {
		httpPort = "8080"
	}
	if httpPort != sshPort {
		return httpPort
	}

	fallback := "8080"
	if fallback == sshPort {
		fallback = "8081"
	}
	log.Printf("WARNING: PORT (%s) is the same as SSH_PORT (%s); they cannot share a listener. "+
		"Serving HTTP on :%s instead. Set PORT to a different value — on Railway the web "+
		"proxy routes to PORT, so leave it at 8080 and keep SSH_PORT=%s.",
		httpPort, sshPort, fallback, sshPort)
	return fallback
}

// visitorMsg carries the current visitor count to the model
type visitorMsg int64

func teaHandler(s ssh.Session) (tea.Model, []tea.ProgramOption) {
	renderer := bubbletea.MakeRenderer(s)

	// Increment on connect, decrement when the session ends
	visitorCount.Add(1)
	count := visitorCount.Load()
	recordVisit(count)

	// Subscribe to guestbook broadcasts, and tear it down with the session so
	// a long-running server doesn't accumulate dead subscribers.
	notify, unsubscribe := TheGuestbook.Subscribe()
	go func() {
		<-s.Context().Done()
		visitorCount.Add(-1)
		unsubscribe()
	}()

	info := gatherSessionInfo(s)

	m := NewModel(renderer)
	m.visitorCount = count
	m.guestNotify = notify
	m.guestEntries = TheGuestbook.Entries()
	m.isAdmin = isAdminSession(s)
	m.directRoute = info.Direct
	m.client = views.ClientInfo{
		User:     info.User,
		Client:   clientName(info.ClientVer),
		Term:     info.Term,
		Width:    info.Width,
		Height:   info.Height,
		KeyType:  info.KeyType,
		MaskedIP: maskIP(info.RemoteIP),
		Colors:   colorDepth(renderer),
	}
	if info.Width > 0 && info.Height > 0 {
		m.width, m.height = info.Width, info.Height
	}
	m.applyDirectRoute()

	return m, []tea.ProgramOption{
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	}
}

// colorDepth reports how many colours the client's terminal advertises.
func colorDepth(r *lipgloss.Renderer) int {
	switch r.ColorProfile() {
	case termenv.TrueColor:
		return 16777216
	case termenv.ANSI256:
		return 256
	case termenv.ANSI:
		return 16
	default:
		return 0
	}
}
