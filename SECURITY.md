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
- Docker unix socket (container management operations)
- `wttr.in` HTTP API (outbound only)
- Local config file (`~/.config/homedash/config.yaml`)

Security concerns are most relevant around Docker socket access, as it allows container start/stop/restart operations.

## Supported Versions

Only the latest release is supported with security updates.
