package main

import (
	"os"
	"runtime"
	"strings"
	"sync/atomic"
	"time"

	"github.com/charmbracelet/ssh"
	gossh "golang.org/x/crypto/ssh"

	"github.com/trafalgar-2006/ssh-portfolio/views"
)

// Traffic counters. visitorCount (live sessions) lives in main.go; these track
// cumulative and peak figures for the admin dashboard.
var (
	totalVisits  atomic.Int64
	peakSessions atomic.Int64
)

// recordVisit bumps the cumulative counter and the concurrent-session
// high-water mark.
func recordVisit(live int64) {
	totalVisits.Add(1)
	for {
		peak := peakSessions.Load()
		if live <= peak || peakSessions.CompareAndSwap(peak, live) {
			return
		}
	}
}

// adminFingerprints returns the set of SSH public key fingerprints allowed to
// see the admin dashboard, from ADMIN_SSH_KEYS.
//
// Accepts either full authorized_keys lines or bare SHA256: fingerprints,
// comma- or newline-separated. Empty means the admin view is disabled.
func adminFingerprints() map[string]bool {
	raw := os.Getenv("ADMIN_SSH_KEYS")
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	out := map[string]bool{}
	for _, field := range strings.FieldsFunc(raw, func(r rune) bool { return r == ',' || r == '\n' }) {
		field = strings.TrimSpace(field)
		if field == "" {
			continue
		}
		if strings.HasPrefix(field, "SHA256:") {
			out[field] = true
			continue
		}
		// Parse as an authorized_keys line and derive its fingerprint.
		if pk, _, _, _, err := gossh.ParseAuthorizedKey([]byte(field)); err == nil {
			out[gossh.FingerprintSHA256(pk)] = true
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// isAdminSession reports whether this session authenticated with an operator
// key. Returns false when ADMIN_SSH_KEYS is unset, so the dashboard is opt-in.
func isAdminSession(s ssh.Session) bool {
	allowed := adminFingerprints()
	if len(allowed) == 0 {
		return false
	}
	key := s.PublicKey()
	if key == nil {
		return false // keyboard-interactive sessions offer no key
	}
	return allowed[gossh.FingerprintSHA256(key)]
}

// collectAdminStats assembles the live server picture.
func collectAdminStats() views.AdminStats {
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)

	lastRun, lastErr, added, _, rate := gitHubSync.snapshot()
	errText := ""
	if lastErr != nil {
		errText = lastErr.Error()
	}
	today, week, top := waka.snapshot()

	return views.AdminStats{
		Uptime:        time.Since(startedAt),
		Sessions:      visitorCount.Load(),
		PeakSessions:  peakSessions.Load(),
		TotalVisits:   totalVisits.Load(),
		Guestbook:     TheGuestbook.Count(),
		Subscribers:   TheGuestbook.Subscribers(),
		GoVersion:     runtime.Version(),
		Commit:        BuildCommit,
		Built:         BuildDate,
		NumGoroutine:  runtime.NumGoroutine(),
		HeapMB:        float64(ms.HeapAlloc) / (1024 * 1024),
		SysMB:         float64(ms.Sys) / (1024 * 1024),
		GCRuns:        ms.NumGC,
		Projects:      views.ProjectCount(),
		SyncedRepos:   added,
		LastSync:      lastRun,
		LastSyncErr:   errText,
		RateRemaining: rate,
		WakaToday:     today,
		WakaWeek:      week,
		WakaTop:       top,
	}
}
