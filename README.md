# ssh-portfolio

> An interactive TUI portfolio served over raw SSH **and** a web version at [mohith.is-a.dev](https://mohith.is-a.dev). No frameworks. No loading spinners.

## 👾 Connect

```bash
# One-time install — drops a `mohith` command on your machine
curl -fsSL https://raw.githubusercontent.com/Trafalgar-2006/portfolio/master/install.sh | bash

# Then just run anytime
mohith
```

```bash
# Or connect directly (no install needed)
ssh ssh.mohith.is-a.dev -p 41074
```

### Jump straight to a screen

Pass a command and the intro is skipped entirely:

```bash
ssh ssh.mohith.is-a.dev -p 41074 projects   # also: about, resume, now, contacts
ssh ssh.mohith.is-a.dev -p 41074 guestbook  # sign the shared wall
ssh ssh.mohith.is-a.dev -p 41074 fx         # the effects playground
ssh ssh.mohith.is-a.dev -p 41074 snake      # or tetris
```

---

## What visitors see

1. **Matrix rain** — katakana + digits fall across the full terminal
2. **Name solidification** — ASCII name crystallises out of the chaos, cell by cell
3. **Boot sequence** — fake SSH handshake, corrupted loading line, signal-lost recovery
4. **"Unauthorized Access" alert** — red full-screen warning → "just kidding. welcome. :)"
5. **Home splash** — braille portrait, name glitch, typewriter tagline, live GitHub commit
6. **Projects** — split-pane, cascade drop-in, live-status pulse, decrypt-reveal descriptions
7. **About / Resume / /now** — everything driven from `content.yaml`
8. **Guestbook** — a shared wall; messages appear live in every connected session

---

## Controls

### Everywhere

| Key | Action |
|---|---|
| `← →` / `h l` | Switch tabs (home) |
| `Enter` | Open the selected tab |
| `↑ ↓` / `j k` | Scroll, or move the project cursor |
| `PgUp` / `PgDn`, `Ctrl+U` / `Ctrl+D` | Page up/down |
| `gg` / `G` | Jump to top / bottom |
| `/` or `Ctrl+K` | Command palette (fuzzy jump to anything) |
| `t` | Cycle colour theme |
| `s` | Effects playground / screensaver |
| `w` | Toggle sine-wave distortion |
| `Esc` | Back |
| `q` `q` | Quit (press twice to confirm) |

### Effects playground (`s`)

| Key | Action |
|---|---|
| `n` / `p` | Next / previous effect |
| `l` | Lock the current effect (stop auto-cycling) |
| *mouse click* | Interact — pour sand, stamp a glider, ignite fire, steer the tunnel |
| any other key | Exit |

It also starts on its own after 45 seconds idle.

### Games

`snake` and `tetris` from the command palette. Arrows or `hjkl`; `space` to hard-drop in Tetris; `p` pause, `r` restart, `esc` back.

---

## Effects

Eight full-screen effects, all rendered through a shared clipping canvas with
run-length grouped ANSI output so a flat row costs one escape sequence:

`digital rain` · `fire` (heat-map demoscene fire) · `torus` (3D, real surface
normals + depth shading) · `plasma` · `tunnel` · `game of life` ·
`falling sand` · `wireframe cube`

Plus five text effects used for headings and page transitions: `slot machine`,
`swarm`, `spotlight`, `blackhole`, and a `particle splatter` that shatters the
outgoing page into falling debris.

---

## Stack

| Layer | Tech |
|---|---|
| TUI framework | [Bubbletea](https://github.com/charmbracelet/bubbletea) |
| SSH server | [Wish](https://github.com/charmbracelet/wish) + [charmbracelet/ssh](https://github.com/charmbracelet/ssh) |
| Styling | [Lipgloss](https://github.com/charmbracelet/lipgloss) |
| Web portfolio | Vanilla HTML/CSS/JS — embedded in the Go binary via `go:embed` |
| Custom domain | `mohith.is-a.dev` (web) · `ssh.mohith.is-a.dev` (SSH) via [is-a.dev](https://is-a.dev) |
| Keep-alive | UptimeRobot HTTP + TCP monitor |

---

## Configuration

All of it optional except the SSH basics — see `.env.example` for the full list.

| Variable | Default | Purpose |
|---|---|---|
| `SSH_ENABLED` | `false` | `true` starts the SSH server; `false` runs the TUI locally |
| `SSH_PORT` | `23234` | SSH listener |
| `PORT` | `8080` | HTTP listener (most PaaS platforms inject this) |
| `SSH_HOST_KEY` | — | base64 ed25519 key, so the fingerprint survives redeploys |
| `GITHUB_USER` | `trafalgar-2006` | Account the project sync polls |
| `GITHUB_TOKEN` | — | Optional PAT; raises the API limit from 60/h to 5000/h |
| `GITHUB_SYNC` | `true` | `false` disables the sync worker |
| `SYNC_INTERVAL` | `1h` | How often to poll GitHub |
| `GUESTBOOK_PATH` | `data/guestbook.jsonl` | Mount on a volume to persist across deploys |
| `ADMIN_SSH_KEYS` | — | Keys allowed to see live server stats. Unset = admin view disabled |
| `WAKATIME_API_KEY` | — | Enables the coding-activity panel; omitted entirely when unset |

**Generate a persistent host key:**
```bash
ssh-keygen -t ed25519 -f host_key
base64 -w0 host_key       # paste into SSH_HOST_KEY
```

---

## Auto-updating projects

A background worker polls the GitHub API and merges your public repos into the
project list, so **pushing a new repo makes it appear without a redeploy**.

`content.yaml` stays authoritative:

- a curated project is never overwritten by a repo of the same name
- matching is punctuation-insensitive (`BioAcoustic-Frog-Classifier` matches
  a curated "BioAcoustic Frog Classifier")
- re-syncs are idempotent, and a repo deleted upstream drops out
- forks and archived repos are skipped
- status is derived from push recency (Live / WIP / Research)

---

## Customization

Everything user-facing lives in `content.yaml` — profile, education, experience,
skills, projects, contacts and the `/now` page. No Go edits needed:

```yaml
profile:
  name: "Your Name"
  tagline: "What you do"

experience:
  - title: "Engineer"
    org: "Somewhere"
    period: "2025 – Present"
    bullets: ["Shipped a thing"]

projects:
  - title: "Your Project"
    description: "What it does and why it matters."
    tags: [Go, Python, Docker]
    status: Live        # Live | WIP | Research
    github: "github.com/you/repo"
    highlight: "optional metric"
```

The ASCII name banner and braille portrait live in `views/home.go`;
colour themes in `views/theme.go`.

---

## Architecture

```
main.go          — dual-port: SSH + HTTP (web portfolio, JSON /health)
model.go         — Bubbletea model; one 20fps ticker drives every animation
session.go       — SSH client introspection + `ssh host <command>` routing
screensaver.go   — effects playground / idle screensaver
github_sync.go   — background worker merging public repos into the project list
guestbook.go     — shared message wall, JSONL-persisted, broadcast to sessions
stats.go         — traffic counters + admin key gating
wakatime.go      — optional coding-activity worker
views/
  fx.go          — canvas primitive, Effect/TextEffect interfaces
  fx_screen.go   — fire, torus, plasma, tunnel, life, sand, cube, rain
  fx_text.go     — slot, swarm, spotlight, blackhole, splatter, sine wave
  games.go       — Snake and Tetris
  layout.go      — viewport clipping, ANSI-safe wrap/fit helpers
  content.go     — content.yaml → render-ready structs (with fallbacks)
  theme.go       — 5 colour themes
  ...            — one file per screen
config/loader.go — YAML parser
content.yaml     — all content (edit this, not the Go files)
entrypoint.sh    — decodes SSH_HOST_KEY → persistent key at startup
```

**Tests:** `go test ./...` — every view is rendered at every terminal width
from 0 to 130 to prove it can't panic or overflow, effects are checked for
exact frame dimensions, and the game/guestbook/sync logic is covered directly.

---

## License

MIT
