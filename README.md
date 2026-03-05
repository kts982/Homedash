# HomeDash

[![CI](https://github.com/kts982/Homedash/actions/workflows/ci.yml/badge.svg)](https://github.com/kts982/Homedash/actions/workflows/ci.yml)
[![Go](https://img.shields.io/badge/Go-1.25+-00ADD8?logo=go&logoColor=white)](https://go.dev)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

A terminal dashboard for homelab servers. Built with [Go](https://go.dev) and the [Charm](https://charm.sh) stack.

Zero external dependencies for data collection — reads directly from `/proc`, the Docker unix socket, and [wttr.in](https://wttr.in).

<img width="800" alt="HomeDash dashboard" src="https://github.com/user-attachments/assets/a0d1e394-2b98-4159-8850-6e8c2364bafa" />

<p>
<img width="395" alt="Container logs" src="https://github.com/user-attachments/assets/96204e08-b48d-443c-8b75-9432ff2c00c2" />
<img width="395" alt="Container actions" src="https://github.com/user-attachments/assets/a0e36abc-a5a9-4cc1-8fe1-78153c642e77" />
</p>

## Features

- **System metrics** — CPU (with sparkline history), RAM, disk usage, network I/O, uptime
- **Docker containers** — grouped by compose stack, collapsible, live CPU/memory stats
- **Container detail view** — full-screen log viewer with follow mode, port mappings, mounts, start/stop/restart actions
- **Container search** — filter containers by name with `/`
- **Quick-action menu** — `space` to open, manage containers without leaving the dashboard
- **Notifications** — non-intrusive bar for Docker state changes, disk warnings, weather errors
- **Weather** — current conditions via [wttr.in](https://wttr.in)
- **Mouse support** — click to select, scroll wheel navigation, double-click to open detail view
- **Themes** — Tokyo Night (default), Catppuccin Mocha, Dracula
- **Responsive layout** — adapts from ~40 to 200+ column terminals
- **State persistence** — collapsed stack groups remembered across sessions

## Install

### From source

Requires [Go 1.25+](https://go.dev/dl/) and Linux.

```bash
git clone https://github.com/kts982/Homedash.git
cd Homedash
make build
./homedash
```

### Requirements

- **Linux** (reads from `/proc`)
- **Docker socket** accessible at `/var/run/docker.sock` (no sudo needed if your user is in the `docker` group)
- **Optional**: Internet access for weather via [wttr.in](https://wttr.in)

## Configuration

HomeDash uses a YAML config file at `~/.config/homedash/config.yaml`. All fields are optional — sensible defaults are used when omitted. Unknown fields are rejected to catch typos.

See [`config.example.yaml`](config.example.yaml) for a full annotated example.

### Config Reference

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `theme` | string | `tokyo-night` | Color theme: `tokyo-night`, `catppuccin`, `dracula` |
| `system.disks` | list | `[{path: "/"}]` | Disk mount points to monitor |
| `system.disks[].path` | string | required | Absolute path to mount point |
| `system.disks[].label` | string | same as path | Display label |
| `refresh.system` | duration | `2s` | System metrics refresh interval (min: `1s`) |
| `refresh.docker` | duration | `5s` | Docker stats refresh interval (min: `3s`) |
| `refresh.weather` | duration | `5m` | Weather refresh interval (min: `1m`) |
| `docker.host` | string | `unix:///var/run/docker.sock` | Docker daemon socket |

The Docker host can also be set via the `DOCKER_HOST` environment variable, which takes precedence over the config file.

### Minimal Config

```yaml
# Just override what you need
theme: dracula
system:
  disks:
    - path: /
    - path: /mnt/storage
      label: storage
```

## Key Bindings

### Dashboard

| Key | Action |
|-----|--------|
| `tab` / `shift+tab` | Cycle focused panel |
| `j` / `k` or `Up` / `Down` | Select container / group |
| `enter` | Expand/collapse stack group, or open detail view |
| `space` | Open quick-action menu for selected container |
| `/` | Search / filter containers |
| `s` | Stop container (with confirmation) |
| `S` | Start container (with confirmation) |
| `R` | Restart container (with confirmation) |
| `r` | Force refresh all data |
| `q` / `ctrl+c` | Quit |

### Container Detail View

| Key | Action |
|-----|--------|
| `esc` / `q` | Back to dashboard |
| `j` / `k` or `Up` / `Down` | Scroll logs |
| `g` / `G` | Jump to top / bottom of logs |
| `f` | Toggle log follow mode (live streaming) |
| `l` | Refresh logs |
| `s` | Stop container |
| `S` | Start container |
| `R` | Restart container |

### Mouse

| Action | Effect |
|--------|--------|
| Click container row | Select container |
| Click group header | Toggle collapse |
| Double-click container | Open detail view |
| Scroll wheel | Scroll container list or logs |

## How It Works

```
/proc/stat, /proc/meminfo, ...  ──2s──>  System panel
/var/run/docker.sock (API v1.47) ──5s──>  Container list + stats
wttr.in JSON API               ──5min──>  Weather panel
```

All data collection is tick-driven through [Bubble Tea](https://github.com/charmbracelet/bubbletea) commands — no background goroutines or channels. Docker container stats are fetched in parallel with a 5-worker pool.

Containers are grouped by the `com.docker.compose.project` label, so any compose-based setup works automatically. Standalone containers appear ungrouped.

## Project Structure

```
cmd/homedash/           Entry point
internal/collector/     Data collection (system, docker, weather)
internal/config/        YAML config loader
internal/state/         Persistent UI state
internal/ui/            Bubble Tea UI layer
  components/           Reusable primitives (gauge, sparkline, panel)
  panels/               Screen sections (system, containers, detail, weather, help)
  styles/               Theme palettes
```

## Built With

- [Go](https://go.dev) — language
- [Bubble Tea](https://github.com/charmbracelet/bubbletea) — TUI framework
- [Bubbles](https://github.com/charmbracelet/bubbles) — TUI components
- [Lip Gloss](https://github.com/charmbracelet/lipgloss) — terminal styling
- [bubbletea-overlay](https://github.com/rmhubbert/bubbletea-overlay) — popup compositing

## Troubleshooting

**"Cannot connect to Docker"** — Ensure the Docker socket exists and your user has access:
```bash
ls -la /var/run/docker.sock
# If permission denied, add yourself to the docker group:
sudo usermod -aG docker $USER
# Then log out and back in
```

**No weather data** — Requires outbound HTTP access to `wttr.in`. Weather will retry automatically on failure.

**Disk not showing** — Check that the mount path in your config is correct and accessible. Inaccessible paths show a warning notification instead of silently failing.

**High CPU usage** — Try increasing refresh intervals in config:
```yaml
refresh:
  system: 5s
  docker: 10s
```

## License

MIT — see [LICENSE](LICENSE).
