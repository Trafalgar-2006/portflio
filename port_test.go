package main

import (
	"strings"
	"testing"
)

// HTTP and SSH must never be told to bind the same port. Regression for the
// Railway crash loop: PORT was left at 23234 from the old config, HTTP tried
// to bind the SSH port, the bind failed, and the process exited on every boot.
func TestResolveHTTPPortNeverCollidesWithSSH(t *testing.T) {
	cases := []struct {
		name, rawPort, sshPort, want string
	}{
		{"normal split", "8080", "23234", "8080"},
		{"unset defaults to 8080", "", "23234", "8080"},
		{"whitespace treated as unset", "   ", "23234", "8080"},
		{"platform-assigned port", "3000", "23234", "3000"},

		// The exact production failure.
		{"railway stale config", "23234", "23234", "8080"},
		// And the pathological case where the fallback itself collides.
		{"ssh already on 8080", "8080", "8080", "8081"},
	}
	for _, tc := range cases {
		got := resolveHTTPPort(tc.rawPort, tc.sshPort)
		if got != tc.want {
			t.Errorf("%s: resolveHTTPPort(%q, %q) = %q, want %q",
				tc.name, tc.rawPort, tc.sshPort, got, tc.want)
		}
		if got == tc.sshPort {
			t.Errorf("%s: resolved HTTP port %q collides with SSH port %q",
				tc.name, got, tc.sshPort)
		}
	}
}

// Whatever the input, the result is never the SSH port.
func TestResolveHTTPPortFuzz(t *testing.T) {
	ports := []string{"", " ", "80", "443", "3000", "8080", "8081", "23234", "41074", "65535"}
	for _, raw := range ports {
		for _, ssh := range ports {
			if ssh == "" || ssh == " " {
				continue
			}
			if got := resolveHTTPPort(raw, ssh); got == ssh {
				t.Errorf("resolveHTTPPort(%q, %q) returned the SSH port %q", raw, ssh, got)
			}
		}
	}
}

// GITHUB_EXCLUDE keeps scratch repos off the public portfolio.
func TestExcludedRepos(t *testing.T) {
	t.Setenv("GITHUB_EXCLUDE", "")
	if got := excludedRepos(); got != nil {
		t.Errorf("unset GITHUB_EXCLUDE should exclude nothing, got %v", got)
	}

	t.Setenv("GITHUB_EXCLUDE", "sar2eo-workspace, Info.Slide ,rapidfire")
	ex := excludedRepos()
	for _, name := range []string{"sar2eo-workspace", "info.slide", "RAPIDFIRE", "Sar2eo-Workspace"} {
		if !ex[strings.ToLower(name)] {
			t.Errorf("%q should be excluded (case-insensitive)", name)
		}
	}
	if ex["embedgen"] {
		t.Error("an unlisted repo was wrongly excluded")
	}
}
