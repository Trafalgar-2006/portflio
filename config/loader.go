package config

import (
	"log"
	"os"

	"gopkg.in/yaml.v3"
)

// Project maps to the projects section of content.yaml
type Project struct {
	Title       string   `yaml:"title"`
	Description string   `yaml:"description"`
	Tags        []string `yaml:"tags"`
	Status      string   `yaml:"status"`
	GitHubURL   string   `yaml:"github"`
	Highlight   string   `yaml:"highlight"`
	// Pinned projects are never replaced by the GitHub auto-sync worker.
	Pinned bool `yaml:"pinned"`
}

// Contact maps to the contacts section of content.yaml
type Contact struct {
	Icon  string `yaml:"icon"`
	Label string `yaml:"label"`
	Value string `yaml:"value"`
}

// Role is one entry in the experience timeline.
type Role struct {
	Title    string   `yaml:"title"`
	Org      string   `yaml:"org"`
	Period   string   `yaml:"period"`
	Location string   `yaml:"location"`
	Bullets  []string `yaml:"bullets"`
}

// Education is a degree or programme.
type Education struct {
	School  string   `yaml:"school"`
	Degree  string   `yaml:"degree"`
	Period  string   `yaml:"period"`
	Note    string   `yaml:"note"`
	Bullets []string `yaml:"bullets"`
}

// Skill is a single labelled proficiency bar.
type Skill struct {
	Name     string `yaml:"name"`
	Pct      int    `yaml:"pct"`
	Category string `yaml:"category"` // lang | ml | frame | infra
}

// SkillGroup is a resume-style category listing.
type SkillGroup struct {
	Category string `yaml:"category"`
	Items    string `yaml:"items"`
}

// Profile is the identity block shared by About and Resume.
type Profile struct {
	Name     string `yaml:"name"`
	Tagline  string `yaml:"tagline"`
	Location string `yaml:"location"`
	Headline string `yaml:"headline"`
	Drive    []string `yaml:"drive"`
	ResumeURL string `yaml:"resume_url"`
}

// Now is the /now page content.
type Now struct {
	Building []string `yaml:"building"`
	Learning []string `yaml:"learning"`
	Reading  []string `yaml:"reading"`
	Status   []string `yaml:"status"`
}

// Content holds the entire loaded content.yaml
type Content struct {
	Profile     Profile      `yaml:"profile"`
	Education   []Education  `yaml:"education"`
	Experience  []Role       `yaml:"experience"`
	Skills      []Skill      `yaml:"skills"`
	SkillGroups []SkillGroup `yaml:"skill_groups"`
	Projects    []Project    `yaml:"projects"`
	Contacts    []Contact    `yaml:"contacts"`
	Now         Now          `yaml:"now"`
}

// Loaded holds the parsed content — populated by Load()
var Loaded *Content

// Load reads and parses content.yaml from the given path
func Load(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var c Content
	if err := yaml.Unmarshal(data, &c); err != nil {
		return err
	}
	Loaded = &c
	log.Printf("Loaded content.yaml: %d projects, %d contacts, %d roles, %d skills",
		len(c.Projects), len(c.Contacts), len(c.Experience), len(c.Skills))
	return nil
}
