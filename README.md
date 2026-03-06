# HomeDash

[![CI](https://github.com/kts982/Homedash/actions/workflows/ci.yml/badge.svg)](https://github.com/kts982/Homedash/actions/workflows/ci.yml)
[![Go](https://img.shields.io/badge/Go-1.25+-00ADD8?logo=go&logoColor=white)](https://go.dev)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

A terminal dashboard for single-host Linux homelabs.

HomeDash combines host metrics, Docker Compose stacks, stack summaries, container logs, and common stack or container actions in one TUI. It is built for people running a personal server who want one operational view instead of jumping between `docker ps`, `docker logs`, `htop`, and ad-hoc scripts.

It reads system data from `/proc`, talks directly to the Docker socket, and optionally fetches weather from [wttr.in](https://wttr.in).

> Status: early-stage, Linux-only, source install for now.

## Who It's For

- people running a personal or home Linux server
- Docker Compose users who think in stacks more than raw Docker objects
- users who prefer terminal workflows over web dashboards

## Non-Goals

- replacing `lazydocker` as a general Docker admin console
- managing clusters, Kubernetes, or multi-host fleets
- being a generic monitoring platform

## Screenshots

### Dashboard Overview

<img width="800" alt="HomeDash dashboard overview" src="docs/screenshots/dashboard-overview.png" />

Host metrics, Compose-stack grouping, and container state in one view.

### Container Detail And Actions

<p>
<img width="395" alt="HomeDash container detail view" src="docs/screenshots/container-detail.png" />
<img width="395" alt="HomeDash quick actions menu" src="docs/screenshots/quick-actions.png" />
</p>

Full-screen logs, container metadata, and quick actions without leaving the TUI.

## Features

- **Unified homelab view** - host metrics, Docker containers, and quick actions in one terminal UI
- **Compose-stack grouping and summaries** - containers grouped by `com.docker.compose.project`, with collapsible stacks, health counts, and aggregate stack status
- **Container detail view** - full-screen log viewer with follow mode, port mappings, mounts, and start/stop/restart actions
- **Quick-action menu** - `space` opens fast stack or container actions without leaving the dashboard
- **System metrics** - CPU, RAM, disk usage, network I/O, uptime, and sparkline history
- **Container search** - filter containers by name with `/`
- **Notifications** - Docker state changes, disk warnings, and weather errors
- **Weather** - current conditions via [wttr.in](https://wttr.in)
- **Responsive layout** - works across narrow and wide terminals
- **State persistence** - collapsed stack groups are remembered across sessions
- **Themes and mouse support** - Tokyo Night, Catppuccin, Dracula, plus click and scroll navigation

## Status

HomeDash is early-stage, but usable for day-to-day homelab monitoring and container operations.

Current scope:

- Linux only
- single host only
- Docker and Docker Compose focused
- source install first

Expect ongoing UI and feature changes while the project settles.

## Roadmap

Near term:

- more detail-view polish for logs and metadata
- stack-level log workflows
- packaging and release improvements
- more test coverage around UI layout and Docker edge cases

Not planned:

- Kubernetes support
- multi-host orchestration
- generic Docker object management beyond the homelab workflow

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
| `space` | Open quick-action menu for selected container or stack |
| `/` | Search / filter containers |
| `s` | Stop selected container or stack (with confirmation) |
| `S` | Start selected container or stack (with confirmation) |
| `R` | Restart selected container or stack (with confirmation) |
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

Most data collection is tick-driven through [Bubble Tea](https://github.com/charmbracelet/bubbletea) commands. Docker container stats are fetched in parallel with a 5-worker pool, and log follow mode uses a streaming goroutine tied to the active detail view.

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
