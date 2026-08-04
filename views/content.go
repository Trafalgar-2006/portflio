package views

import "github.com/trafalgar-2006/ssh-portfolio/config"

// This file mirrors config.Content into package-level vars the render functions
// read. Everything has a hardcoded fallback so the binary still renders a
// complete portfolio if content.yaml is missing or unreadable.

// Profile is the identity block shared by About and Resume.
type Profile struct {
	Name      string
	Tagline   string
	Location  string
	Headline  string
	Drive     []string
	ResumeURL string
}

// Role is one entry in the experience timeline.
type Role struct {
	Title    string
	Org      string
	Period   string
	Location string
	Bullets  []string
}

// Education is a degree or programme.
type Education struct {
	School  string
	Degree  string
	Period  string
	Note    string
	Bullets []string
}

// Skill is a labelled proficiency bar.
type Skill struct {
	Name     string
	Pct      int
	Category string // lang | ml | frame | infra
}

// SkillGroup is a resume-style category listing.
type SkillGroup struct {
	Category string
	Items    string
}

// NowContent is the /now page body.
type NowContent struct {
	Building []string
	Learning []string
	Reading  []string
	Status   []string
}

var (
	TheProfile = Profile{
		Name:      "Mohith Akshay Duggirala",
		Tagline:   "ML Engineer · Full-Stack Developer · Builder",
		Location:  "Bengaluru, Karnataka, India",
		Headline:  "AI/ML Engineer · Full-Stack Developer · Computer Vision",
		ResumeURL: "https://github.com/Trafalgar-2006/portfolio/raw/master/Mohith_Akshay_Duggirala_Resume.pdf",
		Drive: []string{
			"Shipping things that actually work in production —",
			"from satellite CV at ISRO, to a live autonomous",
			"trading system, to this SSH portfolio you're in now.",
		},
	}

	AllEducation = []Education{{
		School: "Manipal Institute of Technology, Bengaluru",
		Degree: "B.Tech — Electronics & Computer Engineering",
		Period: "Aug 2023 – Jul 2027",
		Note:   "GPA: In Progress",
		Bullets: []string{
			"President, MBOSC (Manipal Bengaluru Open Source Community)",
			"Coursework: DSA, Algorithms, Network Protocols, Embedded Systems",
		},
	}}

	AllExperience = []Role{
		{
			Title: "Computer Vision Intern", Org: "ISRO – LEOS",
			Period: "Dec 2025 – Jan 2026", Location: "Bengaluru",
			Bullets: []string{
				"BlenderProc pipeline → 6,000+ COCO/YOLO annotated images",
				"YOLOv7 on Jetson Xavier via TensorRT — 22 FPS real-time",
				"GAN domain adaptation to close sim-to-real gap",
			},
		},
		{
			Title: "Software Engineering Intern", Org: "SenseOps Tech Solutions",
			Period: "May – Jul 2025", Location: "Bengaluru",
			Bullets: []string{
				"Rebuilt portfolio: mobile-first, Lighthouse gains",
				"Custom UDP/TCP packet analyser for vibration sensor network",
			},
		},
		{
			Title: "Founder & Lead Engineer", Org: "Webcraft Studios",
			Period: "Jan 2025 – Present", Location: "Bengaluru",
			Bullets: []string{
				"Digital agency, multiple intl. SaaS clients in year one",
				"React / Node.js / MongoDB / Stripe — subscription billing",
			},
		},
	}

	AllSkills = []Skill{
		{"Python", 95, "lang"}, {"Go", 82, "lang"}, {"TypeScript", 75, "lang"}, {"C / C++", 65, "lang"},
		{"PyTorch", 88, "ml"}, {"TensorFlow", 78, "ml"}, {"YOLOv7", 85, "ml"}, {"TensorRT", 72, "ml"},
		{"React", 80, "frame"}, {"Node.js", 77, "frame"},
		{"Docker", 83, "infra"}, {"AWS", 68, "infra"},
	}

	AllSkillGroups = []SkillGroup{
		{"AI / ML", "PyTorch · TensorFlow · Transformers · LoRA · GGUF · ONNX"},
		{"Languages", "Python · Go · TypeScript · C/C++"},
		{"CV / Vision", "OpenCV · YOLO · Segmentation · Remote Sensing · BlenderProc"},
		{"Web / Backend", "React · Next.js · Node.js · FastAPI · Docker"},
		{"Tools", "Git · Linux · CUDA · Firebase · Oracle Cloud"},
	}

	TheNow = NowContent{
		Building: []string{
			"EmbedGen — LLM fine-tuning on embedded-systems code",
			"Autonomous trading agent (paper trading, live)",
			"This SSH portfolio — always iterating on it",
		},
		Learning: []string{
			"Distributed systems & consensus algorithms (Raft)",
			"Rust for embedded targets",
			"Advanced quantization — AWQ, GPTQ",
		},
		Reading: []string{
			`"Designing Data-Intensive Applications" — Kleppmann`,
			`"The Pragmatic Programmer" — Hunt & Thomas`,
		},
		Status: []string{
			"B.Tech ECE @ Manipal Institute of Technology, Bengaluru (2023–2027)",
			"Open to SWE / ML internships and research roles",
		},
	}
)

// loadNarrativeFromConfig overlays the YAML content onto the fallbacks.
// Each section is only replaced when the YAML actually supplies it, so a
// partial content.yaml degrades to the built-in copy rather than blanking out.
func loadNarrativeFromConfig() {
	c := config.Loaded
	if c == nil {
		return
	}

	if p := c.Profile; p.Name != "" || p.Tagline != "" {
		if p.Name != "" {
			TheProfile.Name = p.Name
		}
		if p.Tagline != "" {
			TheProfile.Tagline = p.Tagline
		}
		if p.Location != "" {
			TheProfile.Location = p.Location
		}
		if p.Headline != "" {
			TheProfile.Headline = p.Headline
		}
		if p.ResumeURL != "" {
			TheProfile.ResumeURL = p.ResumeURL
		}
		if len(p.Drive) > 0 {
			TheProfile.Drive = p.Drive
		}
	}

	if len(c.Education) > 0 {
		AllEducation = AllEducation[:0]
		for _, e := range c.Education {
			AllEducation = append(AllEducation, Education{
				School: e.School, Degree: e.Degree, Period: e.Period,
				Note: e.Note, Bullets: e.Bullets,
			})
		}
	}

	if len(c.Experience) > 0 {
		AllExperience = AllExperience[:0]
		for _, r := range c.Experience {
			AllExperience = append(AllExperience, Role{
				Title: r.Title, Org: r.Org, Period: r.Period,
				Location: r.Location, Bullets: r.Bullets,
			})
		}
	}

	if len(c.Skills) > 0 {
		AllSkills = AllSkills[:0]
		for _, s := range c.Skills {
			pct := s.Pct
			if pct < 0 {
				pct = 0
			}
			if pct > 100 {
				pct = 100
			}
			AllSkills = append(AllSkills, Skill{Name: s.Name, Pct: pct, Category: s.Category})
		}
	}

	if len(c.SkillGroups) > 0 {
		AllSkillGroups = AllSkillGroups[:0]
		for _, g := range c.SkillGroups {
			AllSkillGroups = append(AllSkillGroups, SkillGroup{Category: g.Category, Items: g.Items})
		}
	}

	if n := c.Now; len(n.Building) > 0 || len(n.Learning) > 0 || len(n.Reading) > 0 || len(n.Status) > 0 {
		if len(n.Building) > 0 {
			TheNow.Building = n.Building
		}
		if len(n.Learning) > 0 {
			TheNow.Learning = n.Learning
		}
		if len(n.Reading) > 0 {
			TheNow.Reading = n.Reading
		}
		if len(n.Status) > 0 {
			TheNow.Status = n.Status
		}
	}
}
