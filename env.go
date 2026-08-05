package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"strings"
)

// Local .env support.
//
// Reads a KEY=value file at startup so secrets don't have to be typed into the
// shell on every run. Deliberately a few dozen lines rather than a dependency:
// the format is trivial and the one semantic that matters is the precedence
// rule below.
//
// PRECEDENCE: the real environment always wins over the file.
//
// That direction is not arbitrary. In production (Railway, Oracle Cloud, Fly,
// Docker) the platform injects real environment variables, and a stale .env
// baked into an image must never override them. The file is a convenience for
// local development, not a config source of record.
//
// Supported syntax:
//
//	# comment lines and blank lines are skipped
//	KEY=value
//	export KEY=value          # leading `export` is tolerated
//	KEY="quoted value"        # \n and \" are unescaped inside double quotes
//	KEY='literal value'       # single quotes are literal, no unescaping
//	KEY=                      # sets an explicit empty value

// LoadDotEnv reads path and sets any variable not already present in the
// environment. A missing file is not an error — that's the normal case in
// production, where the platform supplies the environment directly.
//
// Returns the number of variables applied.
func LoadDotEnv(path string) (int, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}
	defer f.Close()

	applied := 0
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024) // room for long key material
	lineNo := 0

	for sc.Scan() {
		lineNo++
		key, val, ok := parseEnvLine(sc.Text())
		if !ok {
			continue
		}
		if key == "" {
			log.Printf(".env:%d: skipping line with an empty key", lineNo)
			continue
		}
		// Real environment wins — see the precedence note above.
		if _, present := os.LookupEnv(key); present {
			continue
		}
		if err := os.Setenv(key, val); err != nil {
			return applied, fmt.Errorf(".env:%d: setting %s: %w", lineNo, key, err)
		}
		applied++
	}
	if err := sc.Err(); err != nil {
		return applied, err
	}
	return applied, nil
}

// parseEnvLine splits one line into a key and value. ok is false for comments,
// blank lines, and anything without an '=' separator.
func parseEnvLine(line string) (key, val string, ok bool) {
	s := strings.TrimSpace(line)
	if s == "" || strings.HasPrefix(s, "#") {
		return "", "", false
	}
	// Tolerate `export KEY=value`, which people paste from shell snippets.
	if rest, found := strings.CutPrefix(s, "export "); found {
		s = strings.TrimSpace(rest)
	}

	eq := strings.Index(s, "=")
	if eq < 0 {
		return "", "", false
	}
	key = strings.TrimSpace(s[:eq])
	val = strings.TrimSpace(s[eq+1:])

	// Quoted values keep their interior whitespace and may carry a trailing
	// comment outside the quotes.
	switch {
	case len(val) >= 2 && val[0] == '"' && strings.LastIndex(val, `"`) > 0:
		end := strings.LastIndex(val, `"`)
		inner := val[1:end]
		// Unescape the sequences that matter for multi-line key material.
		inner = strings.ReplaceAll(inner, `\n`, "\n")
		inner = strings.ReplaceAll(inner, `\"`, `"`)
		val = inner
	case len(val) >= 2 && val[0] == '\'' && strings.LastIndex(val, "'") > 0:
		end := strings.LastIndex(val, "'")
		val = val[1:end] // single quotes are literal
	default:
		// Unquoted: strip a trailing ` # comment`, which is only a comment when
		// preceded by whitespace (so URLs with a fragment survive).
		if i := strings.Index(val, " #"); i >= 0 {
			val = strings.TrimSpace(val[:i])
		}
	}
	return key, val, true
}

// dotEnvPath is the file LoadDotEnv reads. DOTENV_PATH overrides it, which is
// handy for running several local instances from different configs.
func dotEnvPath() string {
	if p := strings.TrimSpace(os.Getenv("DOTENV_PATH")); p != "" {
		return p
	}
	return ".env"
}

// initEnv loads the local .env and reports what happened. Called as the very
// first statement in main(): SSH_ENABLED is read immediately afterwards and
// decides the entire execution path, and WAKATIME_API_KEY / GITHUB_SYNC are
// read once at startup — if they aren't in place by then, those features stay
// off for the life of the process.
func initEnv() {
	path := dotEnvPath()
	n, err := LoadDotEnv(path)
	switch {
	case err != nil:
		log.Printf("Could not read %s: %v (continuing with the process environment)", path, err)
	case n > 0:
		log.Printf("Loaded %d variable(s) from %s", n, path)
	}
	// Silence when the file is absent: that's the normal production case.
}
