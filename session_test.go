package main

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/trafalgar-2006/ssh-portfolio/views"
)

// `ssh host <command>` must map to the right screen, and unknown commands
// must fall through to the normal intro rather than erroring.
func TestRouteCommand(t *testing.T) {
	cases := map[string]string{
		"projects": "projects", "project": "projects", "work": "projects",
		"about": "about", "bio": "about", "me": "about",
		"contact": "contacts", "contacts": "contacts",
		"resume": "resume", "cv": "resume",
		"now":       "now",
		"snake":     "snake",
		"tetris":    "tetris",
		"fx":        "fx",
		"guestbook": "guestbook",
		"admin":     "admin",
		"PROJECTS":  "projects", // case-insensitive
		"  about  ": "about",    // trimmed
	}
	for in, want := range cases {
		if got := routeCommand([]string{in}); got != want {
			t.Errorf("routeCommand(%q) = %q, want %q", in, got, want)
		}
	}
	for _, in := range [][]string{nil, {}, {"nonsense"}, {""}, {"rm -rf /"}} {
		if got := routeCommand(in); got != "" {
			t.Errorf("routeCommand(%v) = %q, want empty", in, got)
		}
	}
}

// A direct route must land on the right view with the intro skipped.
func TestApplyDirectRoute(t *testing.T) {
	cases := []struct {
		route string
		want  View
	}{
		{"projects", ViewProjects},
		{"about", ViewAbout},
		{"contacts", ViewContacts},
		{"resume", ViewResume},
		{"now", ViewNow},
		{"neofetch", ViewNeofetch},
		{"guestbook", ViewGuestbook},
		{"snake", ViewGames},
		{"tetris", ViewGames},
	}
	for _, tc := range cases {
		m := booted(100, 36)
		m.directRoute = tc.route
		m.applyDirectRoute()

		if m.currentView != tc.want {
			t.Errorf("route %q landed on %v, want %v", tc.route, m.currentView, tc.want)
		}
		if m.currentView == ViewMatrix || m.currentView == ViewBoot {
			t.Errorf("route %q did not skip the intro", tc.route)
		}
		// The screen must actually render something.
		if out := strings.TrimSpace(views.StripAnsiForTest(m.View())); out == "" {
			t.Errorf("route %q rendered a blank screen", tc.route)
		}
	}

	// The FX route opens the screensaver rather than a static view.
	m := booted(100, 36)
	m.directRoute = "fx"
	m.applyDirectRoute()
	if !m.saverActive {
		t.Error("route \"fx\" did not start the screensaver")
	}
}

// A non-admin asking for the admin route must get the denial, not the stats.
func TestAdminRouteRequiresKey(t *testing.T) {
	m := booted(100, 36)
	m.isAdmin = false
	m.directRoute = "admin"
	m.applyDirectRoute()
	if m.currentView == ViewAdmin {
		t.Error("non-admin was routed straight into the admin view")
	}

	// Even reaching ViewAdmin directly must render the denial.
	m.currentView = ViewAdmin
	out := views.StripAnsiForTest(m.View())
	if !strings.Contains(strings.ToLower(out), "requires an authorised ssh key") {
		t.Errorf("non-admin did not get the denial screen:\n%s", out)
	}
	if strings.Contains(out, "goroutines") {
		t.Error("non-admin was shown runtime internals")
	}

	// With the flag set, the real dashboard renders.
	m.isAdmin = true
	out = views.StripAnsiForTest(m.View())
	if !strings.Contains(out, "uptime") {
		t.Errorf("admin dashboard did not render for an operator:\n%s", out)
	}
}

// With ADMIN_SSH_KEYS unset the admin view must be entirely disabled.
func TestAdminDisabledByDefault(t *testing.T) {
	t.Setenv("ADMIN_SSH_KEYS", "")
	if fps := adminFingerprints(); fps != nil {
		t.Errorf("admin enabled with no ADMIN_SSH_KEYS: %v", fps)
	}
}

// An authorized_keys line must be accepted and reduced to its fingerprint.
func TestAdminFingerprintParsing(t *testing.T) {
	// A real ed25519 public key line (this is a throwaway test key).
	const key = "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIGb9ECWmEzf6FQbrBZ9w7ZzL0GAqBHCknwmoiwrfDbAP test@example"
	t.Setenv("ADMIN_SSH_KEYS", key)
	fps := adminFingerprints()
	if len(fps) != 1 {
		t.Fatalf("parsed %d fingerprints, want 1", len(fps))
	}
	for fp := range fps {
		if !strings.HasPrefix(fp, "SHA256:") {
			t.Errorf("fingerprint %q is not a SHA256 form", fp)
		}
	}

	// A bare fingerprint must pass through unchanged.
	t.Setenv("ADMIN_SSH_KEYS", "SHA256:abcdef123456")
	if fps := adminFingerprints(); !fps["SHA256:abcdef123456"] {
		t.Errorf("bare fingerprint not accepted: %v", fps)
	}

	// Multiple keys, comma-separated.
	t.Setenv("ADMIN_SSH_KEYS", "SHA256:aaa,SHA256:bbb\nSHA256:ccc")
	if fps := adminFingerprints(); len(fps) != 3 {
		t.Errorf("parsed %d of 3 keys: %v", len(fps), fps)
	}
}

// IP masking must hide the host portion.
func TestMaskIP(t *testing.T) {
	cases := map[string]string{
		"203.0.113.42": "203.0.x.x",
		"":             "unknown",
	}
	for in, want := range cases {
		if got := maskIP(in); got != want {
			t.Errorf("maskIP(%q) = %q, want %q", in, got, want)
		}
	}
	// IPv6 must not leak the full address either.
	if got := maskIP("2001:db8:85a3::8a2e:370:7334"); strings.Contains(got, "7334") {
		t.Errorf("maskIP leaked the IPv6 host portion: %q", got)
	}
}

// The SSH client banner must be trimmed to something readable.
func TestClientName(t *testing.T) {
	cases := map[string]string{
		"SSH-2.0-OpenSSH_9.6":             "OpenSSH_9.6",
		"SSH-2.0-OpenSSH_for_Windows_8.1": "OpenSSH_for_Windows_8.1",
		"":                                "unknown",
	}
	for in, want := range cases {
		if got := clientName(in); got != want {
			t.Errorf("clientName(%q) = %q, want %q", in, got, want)
		}
	}
}

// The neofetch card must render at any size and never leak a raw IP.
func TestNeofetchRenders(t *testing.T) {
	r := lipgloss.DefaultRenderer()
	info := views.ClientInfo{
		User: "guest", Client: "OpenSSH_9.6", Term: "xterm-256color",
		Width: 120, Height: 40, KeyType: "ssh-ed25519",
		MaskedIP: "203.0.x.x", SessionID: "A1B2-C3D4", Connected: "01:23", Colors: 16777216,
	}
	for _, sz := range [][2]int{{0, 0}, {1, 1}, {40, 10}, {80, 24}, {200, 60}} {
		sz := sz
		func() {
			defer func() {
				if rec := recover(); rec != nil {
					t.Fatalf("panic at %dx%d: %v", sz[0], sz[1], rec)
				}
			}()
			views.RenderNeofetch(r, sz[0], sz[1], info, views.ThemeDracula)
		}()
	}
	out := views.StripAnsiForTest(views.RenderNeofetch(r, 120, 40, info, views.ThemeDracula))
	for _, want := range []string{"guest", "OpenSSH_9.6", "xterm-256color", "A1B2-C3D4", "203.0.x.x"} {
		if !strings.Contains(out, want) {
			t.Errorf("neofetch card is missing %q", want)
		}
	}
}
