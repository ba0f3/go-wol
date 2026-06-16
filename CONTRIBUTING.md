# Contributing to go-wol

## Quick Start

```bash
# Install deps
go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest

# Run all checks
make all
```

## Development Workflow

1. Fork and clone
2. Create a feature branch: `git checkout -b feat/my-change`
3. Make changes
4. Run `make all` — must pass (fmt, vet, lint, test, build)
5. Commit with conventional message: `feat: add xyz`
6. Push and open PR

## Code Standards

- **Go 1.22+** (module declares 1.26)
- **No external deps** beyond current (go-nflog, netlink, go-cache)
- **Linux-only** — NFLOG, ipset, netlink are kernel interfaces
- **Root required** — `CAP_NET_ADMIN` + `CAP_NET_RAW`

## Testing

- Unit tests only: `go test ./...`
- No integration tests (requires kernel netlink/ipset)
- Run with `-race` for race detection: `go test -race ./...`

## Commit Messages

Follow [Conventional Commits](https://www.conventionalcommits.org/):

```
feat: add ipset reload command
fix: shutdown race on channel close
docs: update firewall rules for nftables
test: add config validation tests
```

## Pull Request Checklist

- [ ] `make all` passes
- [ ] Tests added/updated for new behavior
- [ ] README updated if user-facing change
- [ ] CHANGELOG.md updated (if applicable)
- [ ] No breaking changes without major version bump

## Reporting Issues

- Search existing issues first
- Include: OS, kernel version, go version
- For bugs: steps to reproduce, expected vs actual
- For features: use case, proposed design