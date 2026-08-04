package main

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/trafalgar-2006/ssh-portfolio/config"
	"github.com/trafalgar-2006/ssh-portfolio/views"
)

// content.yaml must actually drive About / Resume / now — not just parse.
func TestContentYAMLDrivesViews(t *testing.T) {
	if err := config.Load("content.yaml"); err != nil {
		t.Fatalf("content.yaml failed to load: %v", err)
	}
	views.LoadFromConfig()

	c := config.Loaded
	if len(c.Experience) == 0 || len(c.Skills) == 0 || len(c.Education) == 0 {
		t.Fatal("content.yaml is missing experience/skills/education")
	}
	if len(views.AllExperience) != len(c.Experience) {
		t.Errorf("views has %d roles, yaml has %d", len(views.AllExperience), len(c.Experience))
	}
	if len(views.AllSkills) != len(c.Skills) {
		t.Errorf("views has %d skills, yaml has %d", len(views.AllSkills), len(c.Skills))
	}

	r := lipgloss.DefaultRenderer()
	about := views.StripAnsiForTest(views.RenderAbout(r, 120, 60, views.ThemeDracula))
	resume := views.StripAnsiForTest(views.RenderResume(r, 120, 60, views.ThemeDracula))
	now := views.StripAnsiForTest(views.RenderNow(r, 160, 60, "August 2026", views.ThemeDracula))

	// Each role from the YAML must appear in About.
	for _, role := range c.Experience {
		if !strings.Contains(about, role.Org) {
			t.Errorf("About is missing role org %q from content.yaml", role.Org)
		}
	}
	// Skills too.
	for _, s := range c.Skills {
		if !strings.Contains(about, s.Name) {
			t.Errorf("About is missing skill %q", s.Name)
		}
	}
	// Resume must show the profile name and skill groups.
	if !strings.Contains(strings.ToUpper(resume), strings.ToUpper(c.Profile.Name)) {
		t.Errorf("Resume is missing profile name %q", c.Profile.Name)
	}
	for _, g := range c.SkillGroups {
		if !strings.Contains(resume, g.Category) {
			t.Errorf("Resume is missing skill group %q", g.Category)
		}
	}
	// /now must reflect the yaml lists.
	for _, it := range c.Now.Building {
		if !strings.Contains(now, it) {
			t.Errorf("/now is missing building item %q", it)
		}
	}
	for _, it := range c.Now.Learning {
		if !strings.Contains(now, it) {
			t.Errorf("/now is missing learning item %q", it)
		}
	}
	// Nothing may still reference the dead repo name.
	for name, body := range map[string]string{"about": about, "resume": resume, "now": now} {
		if strings.Contains(body, "portflio") {
			t.Errorf("%s still contains the 'portflio' typo", name)
		}
	}
}

// A missing content.yaml must fall back to built-in copy, not blank screens.
func TestFallbackWhenNoConfig(t *testing.T) {
	saved := config.Loaded
	config.Loaded = nil
	defer func() { config.Loaded = saved }()

	r := lipgloss.DefaultRenderer()
	out := views.StripAnsiForTest(views.RenderAbout(r, 120, 60, views.ThemeDracula))
	if !strings.Contains(out, "Manipal") {
		t.Error("About lost its fallback content with no config loaded")
	}
}
