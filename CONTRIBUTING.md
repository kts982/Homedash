# Contributing to HomeDash

Thanks for your interest in contributing! HomeDash is a terminal dashboard for homelab servers, and contributions are welcome.

## Getting Started

### Prerequisites

- **Go 1.25+**
- **Linux** (reads from `/proc` for system metrics)
- **Docker** socket accessible at `/var/run/docker.sock` (or set `DOCKER_HOST`)

### Development Setup

```bash
git clone https://github.com/kts982/homedash.git
cd homedash
make build    # compile
make run      # build + run
```

### Running Tests

```bash
go test ./...
go test -race ./...
```

## Project Structure

```
cmd/homedash/           Entry point
internal/collector/     Data collection (system, docker, weather) — no TUI deps
internal/config/        YAML config loader
internal/state/         Persistent UI state (collapsed stacks)
internal/ui/            Bubble Tea UI layer
  components/           Reusable primitives (gauge, sparkline, panel, ring buffer)
  panels/               Dashboard sections (system, containers, detail, weather, help)
  styles/               Theme palettes
```

## Code Style

- Standard Go formatting (`gofmt`)
- No external dependencies for data collection — raw `/proc` parsing, Docker unix socket HTTP, wttr.in JSON
- All data collection is tick-driven through Bubble Tea commands — no background goroutines or channels
- Keep UI rendering and data collection cleanly separated (collector package has no TUI deps)

## Submitting Changes

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/my-change`)
3. Make your changes
4. Ensure tests pass (`go test ./...`)
5. Ensure the project builds (`make build`)
6. Submit a pull request against `master`

### Commit Messages

Use conventional-style prefixes:

- `feat:` new feature
- `fix:` bug fix
- `refactor:` code restructuring without behavior change
- `test:` adding or updating tests
- `docs:` documentation changes

### What Makes a Good PR

- Focused on a single change
- Tests included for new functionality
- No unrelated changes mixed in
- Description explains **why**, not just **what**

## Reporting Issues

Open an issue with:
- What you expected to happen
- What actually happened
- Your terminal emulator and size
- Go version (`go version`)
- Linux distribution

## License

By contributing, you agree that your contributions will be licensed under the MIT License.
