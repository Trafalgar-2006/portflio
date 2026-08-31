//go:build ignore

// sync_web regenerates the parts of index.html that must stay identical to
// the TUI: the braille portrait, the block-letter name banner, the tagline
// and the five colour themes.
//
// The web page and the terminal drifted apart once — the site ended up with
// its own portrait at a different width and its own hand-typed bio. Rather
// than keep two copies in step by hand, the page's copy is generated from
// the Go source, and views/web_test.go fails if the two ever disagree.
//
// Run it after editing views/home.go or views/theme.go:
//
//	go run tools/sync_web.go
//
// Everything else in index.html is hand-written and is left alone.
package main

import (
	"fmt"
	"log"
	"os"
	"regexp"
	"strings"
)

const (
	artBegin    = "    /* GENERATED:BEGIN"
	artEnd      = "    /* GENERATED:END */"
	themesBegin = "    /* THEMES:BEGIN"
	themesEnd   = "    /* THEMES:END */"
)

// themeKeys is the field order the page expects, matching views.Theme.
var themeKeys = []string{
	"Name", "Primary", "Secondary", "Accent", "Success", "Warning", "Purple",
	"Text", "DimMid", "Dim", "VeryDim", "BoxBorder", "FooterBg", "FooterText",
	"MatrixHead", "MatrixBright", "MatrixMid", "MatrixDim", "MatrixLocked",
	"StarBright", "StarDim",
}

func main() {
	home := read("views/home.go")
	theme := read("views/theme.go")
	page := read("index.html")

	portrait := goStringSlice(home, "portraitArt")
	banner := goStringSlice(home, "nameBanner")
	if len(portrait) == 0 || len(banner) == 0 {
		log.Fatal("could not find portraitArt / nameBanner in views/home.go")
	}

	tagline := ""
	if m := regexp.MustCompile(`TaglineText\s*=\s*"([^"]*)"`).FindStringSubmatch(home); m != nil {
		tagline = m[1]
	}

	var art strings.Builder
	art.WriteString(artBegin + " — tools/sync_web.go; edit views/home.go, not this */\n")
	art.WriteString("    const PORTRAIT = " + jsArray(portrait) + ";\n\n")
	art.WriteString("    const BANNER = " + jsArray(banner) + ";\n\n")
	art.WriteString("    const TAGLINE = " + jsString(tagline) + ";\n")
	art.WriteString(artEnd)

	var themes strings.Builder
	themes.WriteString(themesBegin + " — tools/sync_web.go; edit views/theme.go, not this */\n")
	themes.WriteString("    const THEMES = [\n")
	names := themeRegistryOrder(theme)
	blocks := themeBlocks(theme)
	for i, n := range names {
		f, ok := blocks[n]
		if !ok {
			log.Fatalf("theme %s is in the registry but has no definition", n)
		}
		pairs := make([]string, 0, len(themeKeys))
		for _, k := range themeKeys {
			pairs = append(pairs, lowerFirst(k)+": "+jsString(f[k]))
		}
		line := "      { " + strings.Join(pairs, ", ") + " }"
		if i < len(names)-1 {
			line += ","
		}
		themes.WriteString(line + "\n")
	}
	themes.WriteString("    ];\n")
	themes.WriteString(themesEnd)

	page = replaceRegion(page, artBegin, artEnd, art.String())
	page = replaceRegion(page, themesBegin, themesEnd, themes.String())

	if err := os.WriteFile("index.html", []byte(page), 0o644); err != nil {
		log.Fatalf("write index.html: %v", err)
	}

	fmt.Printf("portrait: %d rows x %d cols\n", len(portrait), width(portrait))
	fmt.Printf("banner:   %d rows x %d cols\n", len(banner), width(banner))
	fmt.Printf("themes:   %d\n", len(names))
	fmt.Println("index.html updated — run `go test ./views/ -run TestWeb` to confirm")
}

func read(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		log.Fatalf("read %s: %v", path, err)
	}
	return string(b)
}

// goStringSlice pulls the literals out of `name = []string{ "a", "b" }`.
func goStringSlice(src, name string) []string {
	m := regexp.MustCompile(name + `\s*=\s*\[\]string\{([\s\S]*?)\n\}`).FindStringSubmatch(src)
	if m == nil {
		return nil
	}
	var out []string
	for _, q := range regexp.MustCompile(`"([^"]*)"`).FindAllStringSubmatch(m[1], -1) {
		out = append(out, q[1])
	}
	return out
}

// themeRegistryOrder returns the theme variable names in the order [t] cycles
// them, which is the registry's order — not the order they're declared.
func themeRegistryOrder(src string) []string {
	m := regexp.MustCompile(`var Themes = \[\]Theme\{([\s\S]*?)\n\}`).FindStringSubmatch(src)
	if m == nil {
		log.Fatal("could not find `var Themes = []Theme{...}`")
	}
	var out []string
	for _, s := range regexp.MustCompile(`(Theme\w+)`).FindAllStringSubmatch(m[1], -1) {
		out = append(out, s[1])
	}
	return out
}

func themeBlocks(src string) map[string]map[string]string {
	out := map[string]map[string]string{}
	re := regexp.MustCompile(`var (Theme\w+) = Theme\{([\s\S]*?)\n\}`)
	field := regexp.MustCompile(`(\w+):\s*"([^"]+)"`)
	for _, m := range re.FindAllStringSubmatch(src, -1) {
		f := map[string]string{}
		for _, kv := range field.FindAllStringSubmatch(m[2], -1) {
			f[kv[1]] = kv[2]
		}
		out[m[1]] = f
	}
	return out
}

// replaceRegion swaps everything between a begin and end marker, inclusive.
func replaceRegion(page, begin, end, with string) string {
	i := strings.Index(page, begin)
	if i < 0 {
		log.Fatalf("index.html has no %q marker", begin)
	}
	j := strings.Index(page[i:], end)
	if j < 0 {
		log.Fatalf("index.html has no %q marker after %q", end, begin)
	}
	return page[:i] + with + page[i+j+len(end):]
}

// jsString emits a JSON-compatible double-quoted literal. views/web_test.go
// parses these back with encoding/json, so the escaping has to be exact.
func jsString(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '"':
			b.WriteString(`\"`)
		case '\\':
			b.WriteString(`\\`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		default:
			if r < 0x20 {
				fmt.Fprintf(&b, `\u%04x`, r)
			} else {
				b.WriteRune(r)
			}
		}
	}
	b.WriteByte('"')
	return b.String()
}

func jsArray(rows []string) string {
	var b strings.Builder
	b.WriteString("[\n")
	for i, r := range rows {
		b.WriteString("      " + jsString(r))
		if i < len(rows)-1 {
			b.WriteByte(',')
		}
		b.WriteByte('\n')
	}
	b.WriteString("    ]")
	return b.String()
}

func lowerFirst(s string) string {
	if s == "" {
		return s
	}
	return strings.ToLower(s[:1]) + s[1:]
}

func width(rows []string) int {
	max := 0
	for _, r := range rows {
		if n := len([]rune(r)); n > max {
			max = n
		}
	}
	return max
}
