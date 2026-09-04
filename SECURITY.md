# Security Policy

## Reporting a Vulnerability

If you discover a security vulnerability in HomeDash, please report it responsibly.

**Do not open a public issue.** Instead, use [GitHub's private vulnerability reporting](https://github.com/kts982/homedash/security/advisories/new).

Please include:
- Description of the vulnerability
- Steps to reproduce
- Potential impact
- Suggested fix (if any)

You should receive a response within 48 hours.

## Scope

HomeDash interacts with:
- `/proc` filesystem (read-only)
- Docker socket (container management operations; a `tcp://` `DOCKER_HOST` is spoken as plain HTTP)
- `wttr.in` HTTP API (outbound only)
- Container registries over HTTPS when you press `u` (Docker Hub, ghcr.io, private registries): manifest `HEAD` requests plus the token exchange, sending credentials read from `~/.docker/config.json` `auths` entries. Credential helpers are detected but never executed
- Container log output and labels, which are rendered in the terminal
- Local config and state files (`~/.config/homedash/config.yaml`, `state.json`)

Security concerns are most relevant around Docker socket access, as it allows container start/stop/restart operations, and around registry credentials leaving the machine.

## Supported Versions

Only the latest release is supported with security updates.
