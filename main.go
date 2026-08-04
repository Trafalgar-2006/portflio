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
	"sync/atomic"
	"syscall"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/ssh"
	"github.com/charmbracelet/wish"
	"github.com/charmbracelet/wish/activeterm"
	"github.com/charmbracelet/wish/bubbletea"
	"github.com/charmbracelet/wish/logging"
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

	// Background worker: keep the project list in step with GitHub.
	syncCtx, cancelSync := context.WithCancel(context.Background())
	defer cancelSync()
	StartGitHubSync(syncCtx)

	// HTTP server: web portfolio at / and health check at /health.
	// Most PaaS platforms (Railway, Render, Fly, Cloud Run) inject the port to
	// bind as $PORT — honour it, or the health check never comes up.
	httpPort := os.Getenv("PORT")
	if httpPort == "" {
		httpPort = "8080"
	}

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
		// Don't swallow this: if the port is taken the health check silently
		// dies and uptime monitoring reports the whole service as down.
		if err := http.ListenAndServe(addr, mux); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("HTTP server error: %v", err)
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

// visitorMsg carries the current visitor count to the model
type visitorMsg int64

func teaHandler(s ssh.Session) (tea.Model, []tea.ProgramOption) {
	renderer := bubbletea.MakeRenderer(s)

	// Increment on connect, decrement when the session ends
	visitorCount.Add(1)
	count := visitorCount.Load()
	go func() {
		<-s.Context().Done()
		visitorCount.Add(-1)
	}()

	m := NewModel(renderer)
	m.visitorCount = count

	return m, []tea.ProgramOption{
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	}
}
