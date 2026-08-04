package main

import (
	"strings"

	"github.com/charmbracelet/ssh"
)

// SessionInfo is what we can learn about a connecting client, used for the
// neofetch-style greeting card and the admin view.
type SessionInfo struct {
	User      string // SSH username they connected as
	ClientVer string // e.g. "SSH-2.0-OpenSSH_9.6"
	Term      string // TERM value
	Width     int
	Height    int
	KeyType   string // public key algorithm, if they offered one
	RemoteIP  string
	Direct    string // view they asked for via `ssh host <command>`, if any
}

// routeCommand maps `ssh mohith.is-a.dev <command>` to a starting view, so
// power users can skip the menus entirely. Unknown commands fall through to
// the normal intro.
func routeCommand(args []string) string {
	if len(args) == 0 {
		return ""
	}
	switch strings.ToLower(strings.TrimSpace(args[0])) {
	case "projects", "project", "work":
		return "projects"
	case "about", "bio", "me":
		return "about"
	case "contact", "contacts", "email":
		return "contacts"
	case "resume", "cv":
		return "resume"
	case "now":
		return "now"
	case "guestbook", "gb", "chat":
		return "guestbook"
	case "snake":
		return "snake"
	case "tetris":
		return "tetris"
	case "fx", "screensaver", "demo":
		return "fx"
	case "stats", "admin":
		return "admin"
	case "help", "?":
		return "help"
	}
	return ""
}

// gatherSessionInfo extracts what the SSH layer knows about this client.
func gatherSessionInfo(s ssh.Session) SessionInfo {
	info := SessionInfo{
		User:   s.User(),
		Direct: routeCommand(s.Command()),
	}

	if pty, _, ok := s.Pty(); ok {
		info.Term = pty.Term
		info.Width = pty.Window.Width
		info.Height = pty.Window.Height
	}
	if key := s.PublicKey(); key != nil {
		info.KeyType = key.Type()
	}
	if addr := s.RemoteAddr(); addr != nil {
		host := addr.String()
		// Strip the port; the IP alone is all the greeting card shows.
		if i := strings.LastIndex(host, ":"); i > 0 {
			host = host[:i]
		}
		info.RemoteIP = strings.Trim(host, "[]")
	}
	if ctx := s.Context(); ctx != nil {
		info.ClientVer = ctx.ClientVersion()
	}
	return info
}

// maskIP keeps the network but hides the host portion, so the greeting card
// can show where someone is connecting from without echoing a full address
// back at them.
func maskIP(ip string) string {
	if ip == "" {
		return "unknown"
	}
	if strings.Contains(ip, ":") { // IPv6
		parts := strings.Split(ip, ":")
		if len(parts) > 2 {
			return parts[0] + ":" + parts[1] + ":··"
		}
		return "··"
	}
	parts := strings.Split(ip, ".")
	if len(parts) == 4 {
		return parts[0] + "." + parts[1] + ".x.x"
	}
	return ip
}

// clientName trims the SSH version banner down to something readable.
func clientName(ver string) string {
	if ver == "" {
		return "unknown"
	}
	v := strings.TrimPrefix(ver, "SSH-2.0-")
	v = strings.TrimPrefix(v, "SSH-1.99-")
	if i := strings.Index(v, " "); i > 0 {
		v = v[:i]
	}
	return v
}
