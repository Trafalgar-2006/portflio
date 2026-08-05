package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeEnvFile(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), ".env")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// The three secrets this project cares about must round-trip intact.
func TestLoadDotEnvRealSecrets(t *testing.T) {
	const adminKey = "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIGb9ECWmEzf6FQbrBZ9w7ZzL0GAqBHCknwmoiwrfDbAP you@laptop"
	path := writeEnvFile(t, `# secrets
GITHUB_TOKEN=ghp_exampleTokenValue1234567890
WAKATIME_API_KEY=waka_11111111-2222-3333-4444-555555555555
ADMIN_SSH_KEYS="`+adminKey+`"
`)
	for _, k := range []string{"GITHUB_TOKEN", "WAKATIME_API_KEY", "ADMIN_SSH_KEYS"} {
		os.Unsetenv(k)
		t.Cleanup(func() { os.Unsetenv(k) })
	}

	n, err := LoadDotEnv(path)
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}
	if n != 3 {
		t.Errorf("applied %d variables, want 3", n)
	}
	if got := os.Getenv("GITHUB_TOKEN"); got != "ghp_exampleTokenValue1234567890" {
		t.Errorf("GITHUB_TOKEN = %q", got)
	}
	if got := os.Getenv("WAKATIME_API_KEY"); !strings.HasPrefix(got, "waka_") {
		t.Errorf("WAKATIME_API_KEY = %q", got)
	}
	if got := os.Getenv("ADMIN_SSH_KEYS"); got != adminKey {
		t.Errorf("ADMIN_SSH_KEYS = %q, want %q", got, adminKey)
	}

	// And the loaded key must actually parse into a usable fingerprint —
	// this is the end-to-end contract with the admin gate.
	fps := adminFingerprints()
	if len(fps) != 1 {
		t.Fatalf("adminFingerprints() produced %d entries from the loaded key", len(fps))
	}
	for fp := range fps {
		if !strings.HasPrefix(fp, "SHA256:") {
			t.Errorf("fingerprint %q is not in SHA256 form", fp)
		}
	}
}

// The real environment must win. A stale .env in a deployed image must never
// override what the platform injects.
func TestRealEnvironmentWinsOverFile(t *testing.T) {
	path := writeEnvFile(t, "PORT=9999\nGITHUB_TOKEN=from_file\n")

	t.Setenv("PORT", "8080") // as a platform would inject it
	os.Unsetenv("GITHUB_TOKEN")
	t.Cleanup(func() { os.Unsetenv("GITHUB_TOKEN") })

	if _, err := LoadDotEnv(path); err != nil {
		t.Fatal(err)
	}
	if got := os.Getenv("PORT"); got != "8080" {
		t.Errorf("the .env overrode a real environment variable: PORT = %q, want 8080", got)
	}
	if got := os.Getenv("GITHUB_TOKEN"); got != "from_file" {
		t.Errorf("an unset variable was not filled from the file: %q", got)
	}
}

// An explicitly empty real variable still counts as set, and must not be
// overwritten — otherwise `PORT= ./app` couldn't disable a file default.
func TestEmptyRealValueStillWins(t *testing.T) {
	path := writeEnvFile(t, "WAKATIME_API_KEY=from_file\n")
	t.Setenv("WAKATIME_API_KEY", "")
	if _, err := LoadDotEnv(path); err != nil {
		t.Fatal(err)
	}
	if got := os.Getenv("WAKATIME_API_KEY"); got != "" {
		t.Errorf("an explicitly empty variable was overwritten with %q", got)
	}
}

// A missing file is the normal production case, not an error.
func TestMissingFileIsNotAnError(t *testing.T) {
	n, err := LoadDotEnv(filepath.Join(t.TempDir(), "nope.env"))
	if err != nil {
		t.Errorf("missing file reported as an error: %v", err)
	}
	if n != 0 {
		t.Errorf("applied %d variables from a missing file", n)
	}
}

// Syntax handling.
func TestParseEnvLine(t *testing.T) {
	cases := []struct {
		line      string
		key, val  string
		ok        bool
	}{
		{"KEY=value", "KEY", "value", true},
		{"  KEY  =  value  ", "KEY", "value", true},
		{"export KEY=value", "KEY", "value", true},
		{`KEY="quoted value"`, "KEY", "quoted value", true},
		{`KEY='literal value'`, "KEY", "literal value", true},
		{`KEY="line1\nline2"`, "KEY", "line1\nline2", true},
		{`KEY="say \"hi\""`, "KEY", `say "hi"`, true},
		{"KEY=", "KEY", "", true},
		{"KEY=value # trailing comment", "KEY", "value", true},
		{`KEY="value # not a comment"`, "KEY", "value # not a comment", true},
		{"URL=https://x.dev/page#anchor", "URL", "https://x.dev/page#anchor", true},
		{"# whole line comment", "", "", false},
		{"", "", "", false},
		{"   ", "", "", false},
		{"NOEQUALSSIGN", "", "", false},
	}
	for _, tc := range cases {
		key, val, ok := parseEnvLine(tc.line)
		if ok != tc.ok || key != tc.key || val != tc.val {
			t.Errorf("parseEnvLine(%q) = (%q, %q, %v), want (%q, %q, %v)",
				tc.line, key, val, ok, tc.key, tc.val, tc.ok)
		}
	}
}

// Malformed content must be skipped, not abort the load.
func TestMalformedLinesAreSkipped(t *testing.T) {
	path := writeEnvFile(t, `
# comment
GOOD_ONE=yes
this line has no equals sign
=novalue

GOOD_TWO=also yes
`)
	for _, k := range []string{"GOOD_ONE", "GOOD_TWO"} {
		os.Unsetenv(k)
		t.Cleanup(func() { os.Unsetenv(k) })
	}
	n, err := LoadDotEnv(path)
	if err != nil {
		t.Fatalf("malformed content aborted the load: %v", err)
	}
	if n != 2 {
		t.Errorf("applied %d variables, want the 2 valid ones", n)
	}
	if os.Getenv("GOOD_ONE") != "yes" || os.Getenv("GOOD_TWO") != "also yes" {
		t.Error("valid lines after a malformed one were not applied")
	}
}

// DOTENV_PATH must redirect the loader.
func TestDotEnvPathOverride(t *testing.T) {
	if got := dotEnvPath(); got != ".env" {
		t.Errorf("default path = %q, want .env", got)
	}
	t.Setenv("DOTENV_PATH", "config/local.env")
	if got := dotEnvPath(); got != "config/local.env" {
		t.Errorf("DOTENV_PATH override = %q", got)
	}
}

// The shipped .env.example must parse cleanly and document every variable the
// code actually reads — otherwise it's a trap rather than a guide.
func TestEnvExampleIsCompleteAndParses(t *testing.T) {
	data, err := os.ReadFile(".env.example")
	if err != nil {
		t.Fatalf("no .env.example in the repo root: %v", err)
	}
	documented := map[string]bool{}
	for _, line := range strings.Split(string(data), "\n") {
		s := strings.TrimSpace(line)
		// Count commented-out placeholders as documented too.
		s = strings.TrimPrefix(s, "# ")
		s = strings.TrimPrefix(s, "#")
		if k, _, ok := parseEnvLine(s); ok && k != "" {
			documented[k] = true
		}
	}

	// Every variable the code reads.
	required := []string{
		"SSH_ENABLED", "HOST", "SSH_PORT", "PORT", "SSH_HOST_KEY",
		"GITHUB_USER", "GITHUB_TOKEN", "GITHUB_SYNC", "SYNC_INTERVAL", "GITHUB_EXCLUDE",
		"GUESTBOOK_PATH", "GUESTBOOK_MAX",
		"ADMIN_SSH_KEYS",
		"WAKATIME_API_KEY", "WAKATIME_INTERVAL",
	}
	for _, k := range required {
		if !documented[k] {
			t.Errorf(".env.example does not document %s, which the code reads", k)
		}
	}
}
