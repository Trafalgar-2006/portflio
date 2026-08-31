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

Needs a real terminal — the server refuses sessions without a PTY, so
`ssh host exit` or piping the output won't work.

### Jump straight to a screen

Pass a command and the whole intro is skipped:

```bash
ssh ssh.mohith.is-a.dev -p 41074 projects    # also: about, contacts, resume, now
ssh ssh.mohith.is-a.dev -p 41074 guestbook   # sign the shared wall
ssh ssh.mohith.is-a.dev -p 41074 fx          # the effects playground
ssh ssh.mohith.is-a.dev -p 41074 snake       # or tetris
ssh ssh.mohith.is-a.dev -p 41074 neofetch    # system info card (alias: help)
```

`admin` is also a route, but it falls back to `neofetch` unless your key is
in `ADMIN_SSH_KEYS`.

---

## The screenplay

What happens, in order, from the moment someone connects:

| # | Screen | What it does | Ends when |
|---|---|---|---|
| 1 | **Matrix rain** | Katakana + digits fall across the full terminal at ~10fps | After 2s the name starts locking in |
| 2 | **Name solidification** | The ASCII name crystallises out of the rain, 6 cells per frame | Every cell is locked, then a 500ms hold |
| 3 | **Boot sequence** | Fake SSH handshake, corrupted loading line, signal-lost recovery — lines appear on a timed schedule | The schedule runs out |
| 4 | **Unauthorized Access** | Red full-screen warning → *"just kidding. welcome. :)"* | Auto-advances |
| 5 | **Home** | Braille portrait beside the block-letter name banner, twinkling stars, typewriter tagline, bio, live commit + session line | Visitor picks a tab |

From home, `← →` moves along the tab bar — **Projects · About · Contacts ·
Resume · /now** — and `Enter` opens one. Every transition is a wipe; some are
a particle splatter that shatters the outgoing page into falling debris.

Everything past that point is reachable two ways: the tab bar, or `/` for the
command palette.

### Every screen

| Screen | Reached by | What's on it |
|---|---|---|
| **Projects** | tab 1 · `ssh … projects` | Split pane, cascade drop-in, live-status pulse, decrypt-reveal descriptions. Auto-synced from GitHub |
| **About** | tab 2 · `ssh … about` | Profile, education, experience, skills — all from `content.yaml` |
| **Contacts** | tab 3 · `ssh … contacts` | Links and handles. `c` toggles copy-friendly mode (strips styling so you can select cleanly) |
| **Resume** | tab 4 · `ssh … resume` | The CV, rendered inline |
| **/now** | tab 5 · `ssh … now` | What he's working on right now |
| **Guestbook** | `/` palette · `ssh … guestbook` | A shared wall. `i` opens the compose box. Messages appear **live in every connected session** |
| **Time travel** | `/` palette | A scrubber across this repo's own commit history. `← →` moves through time |
| **Neofetch** | `ssh … neofetch` | System-info card in the classic neofetch layout |
| **Games** | `/` palette · `ssh … snake` | Snake and Tetris |
| **Effects playground** | `s` · `ssh … fx` | Eight full-screen effects, mouse-interactive |
| **Admin** | `ssh … admin` | Live server stats — uptime, sessions, heap, goroutines, sync status, WakaTime. Gated on `ADMIN_SSH_KEYS` |

Leave it alone for **45 seconds** and the effects playground takes over as a
screensaver. Any key brings it back.

---

## Controls

### Everywhere

| Key | Action |
|---|---|
| `← →` / `h l` | Move along the tab bar (home) · scrub history (time travel) |
| `Enter` | Open the selected tab |
| `↑ ↓` / `j k` | Scroll, or move the project cursor |
| `5j` `12k` | Count prefix — repeat the motion N times |
| `PgUp` `PgDn` · `Ctrl+U` `Ctrl+D` · `Space` | Half-page up/down |
| `gg` / `G` · `Home` / `End` | Jump to top / bottom |
| `/` or `Ctrl+K` | Command palette — fuzzy jump to any screen *or any project by name* |
| `t` | Cycle colour theme (5 of them) |
| `s` | Effects playground |
| `w` | Toggle sine-wave distortion |
| `Esc` | Back to home |
| `q` | Back to home — on home, press **twice within 4s** to quit |
| `Ctrl+C` | Quit |

### Screen-specific

| Where | Key | Action |
|---|---|---|
| Contacts | `c` | Copy-friendly mode |
| Guestbook | `i` | Open the compose box |
| Guestbook compose | `Tab` · `Enter` · `Esc` | Switch name/message field · post · cancel |
| Time travel | `← →` / `h l` | Scrub through commit history |
| Effects | `n` `Space` `→` / `p` `←` | Next / previous effect |
| Effects | `l` | Lock the current effect (stop auto-cycling) |
| Effects | *mouse click* | Pour sand, stamp a glider, ignite fire, steer the tunnel |
| Effects | any other key | Exit |
| Games | arrows / `hjkl` | Move |
| Games | `Space` · `x` / `↑` | Hard-drop · rotate (Tetris) |
| Games | `p` · `r` · `Esc` | Pause · restart · back |

There's a Konami code in there too. `↑ ↑ ↓ ↓ ← → ← → b a`.

---

## Run it yourself

### Locally, as a plain TUI

No SSH, no server — the portfolio just opens in your terminal:

```bash
git clone https://github.com/Trafalgar-2006/portfolio
cd portfolio
go run .
```

With `SSH_ENABLED` unset or `false`, `main()` takes the local path and runs
Bubbletea straight against your terminal in the alternate screen buffer.
This is the fastest loop for working on views.

### Locally, as a real SSH server

```bash
cp .env.example .env      # then fill in whatever you want enabled
SSH_ENABLED=true go run .
```

Then from another terminal:

```bash
ssh localhost -p 23234
```

The web version comes up alongside it on <http://localhost:8080>, with
`/health` and `/api/content` next to it.

### With Docker

```bash
cp .env.example .env
docker compose up --build
```

SSH on `23234`, web on `8080`, and the generated host key is kept in a named
volume so the fingerprint survives restarts.

### Deploying

The `Dockerfile` is a multi-stage build that stamps the commit into the binary
via ldflags, so `/health` reports what's actually running. It works as-is on
Railway, Render, Fly and Cloud Run.

Two things to get right on any host:

1. **Set `SSH_HOST_KEY`** to a base64 ed25519 key, or every redeploy changes
   the fingerprint and returning visitors get
   `REMOTE HOST IDENTIFICATION HAS CHANGED`.
2. **Mount a volume at `data/`**, or the guestbook is wiped on every deploy.

```bash
ssh-keygen -t ed25519 -f host_key -N ""
base64 -w0 host_key       # paste into SSH_HOST_KEY
```

Most PaaS platforms inject `$PORT` for the HTTP listener. If one injects the
SSH port there by mistake, the app detects the collision and falls back rather
than crash-looping — but it's worth setting `PORT=8080` explicitly.

### Tests

```bash
go test ./...                              # the full suite
CGO_ENABLED=1 go test ./... -race          # needs a C toolchain (gcc)
```

Every view is rendered at every terminal width from 0 to 130 to prove it can't
panic or overflow; effects are checked for exact frame dimensions; the game,
guestbook and sync logic are covered directly. CI runs build, vet, gofmt, the
race detector, and a Docker image build with a container smoke test on every
push.

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
| Web portfolio | Vanilla HTML/CSS/JS — embedded in the Go binary via `go:embed`, fed live by `/api/content` |
| Custom domain | `mohith.is-a.dev` (web) · `ssh.mohith.is-a.dev` (SSH) via [is-a.dev](https://is-a.dev) |
| Keep-alive | UptimeRobot HTTP + TCP monitor |

---

## Configuration

All of it optional except `SSH_ENABLED` — see `.env.example` for the annotated
full list. Copy it to `.env` and the app reads it at startup; `.env` is
gitignored, and **real environment variables always win over the file**, so a
stale `.env` baked into an image can't override what your host injects.

| Variable | Default | Purpose |
|---|---|---|
| `SSH_ENABLED` | `false` | `true` starts the SSH server; `false` runs the TUI locally |
| `HOST` | `0.0.0.0` | SSH bind address |
| `SSH_PORT` | `23234` | SSH listener |
| `PORT` | `8080` | HTTP listener (most PaaS platforms inject this) |
| `SSH_HOST_KEY` | — | base64 ed25519 key, so the fingerprint survives redeploys |
| `GITHUB_USER` | `trafalgar-2006` | Account the project sync polls |
| `GITHUB_TOKEN` | — | Optional PAT; **needs no scopes** — it only raises the API limit from 60/h to 5000/h |
| `GITHUB_SYNC` | `true` | `false` disables the sync worker |
| `GITHUB_EXCLUDE` | — | Comma-separated repos to keep off the portfolio |
| `SYNC_INTERVAL` | `1h` | How often to poll GitHub (min 1m) |
| `GUESTBOOK_PATH` | `data/guestbook.jsonl` | Mount on a volume to persist across deploys |
| `GUESTBOOK_MAX` | `500` | Messages kept in memory (older ones stay on disk) |
| `ADMIN_SSH_KEYS` | — | Keys allowed to see live server stats. Unset = admin view disabled |
| `WAKATIME_API_KEY` | — | Enables the coding-activity panel; omitted entirely when unset |
| `WAKATIME_INTERVAL` | `30m` | How often to refresh (min 1m) |
| `DOTENV_PATH` | `.env` | Read a different env file |

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
- `GITHUB_EXCLUDE` keeps scratch repos out

---

## Customization

Everything user-facing lives in `content.yaml` — profile, education, experience,
skills, projects, contacts and the `/now` page. No Go edits needed, and the web
version reads the same file through `/api/content`, so both surfaces update
together:

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
env.go           — .env loader; real env always wins over the file
model.go         — Bubbletea model; one 20fps ticker drives every animation
session.go       — SSH client introspection + `ssh host <command>` routing
api.go           — /api/content, so the web page reads the same content.yaml
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
  timeline.go    — the commit-history scrubber
  layout.go      — viewport clipping, ANSI-safe wrap/fit helpers
  content.go     — content.yaml → render-ready structs (with fallbacks)
  theme.go       — 5 colour themes
  ...            — one file per screen
config/loader.go — YAML parser
content.yaml     — all content (edit this, not the Go files)
index.html       — the web portfolio, embedded into the binary
entrypoint.sh    — decodes SSH_HOST_KEY → persistent key at startup
```

Shared state that background workers write while sessions read — the project
list, the commit timeline, the guestbook — is guarded by `sync.RWMutex` behind
accessor functions, and CI runs the race detector on every push.

---

## License

MIT
