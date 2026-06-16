# go-wol — Agent Guidance

## Quick Commands

```bash
# Build
make build          # outputs to bin/go-wol
go build -o go-wol .  # direct, outputs to project root

# Test & verify
make test           # go test -count=1 ./...
make vet            # go vet ./...
make fmt            # go fmt ./...
make lint           # golangci-lint run (needs install-lint first)

# Run (requires root + env vars)
export IPSET_NAME=lan_hosts
export NFLOG_GROUP=100
export CACHE_TTL=2m
sudo ./go-wol
```

## Architecture Overview

**Purpose**: Linux daemon that wakes LAN hosts via WOL when Tailscale SYN traffic arrives at the router.

**Pipeline**: `tailscale0 → iptables/nftables (NFLOG) → go-wol → ipset (IP→MAC) → netlink (route→vlan) → WOL magic packet (SO_BINDTODEVICE)`

**Packages**:
- `config/` — env var config (`IPSET_NAME` required, others optional with defaults)
- `ipset/` — `hash:ip,mac` netlink lookup + in-memory cache, `Refresh()` on SIGHUP
- `network/` — `netlink.RouteGet` → outbound interface name
- `nflog/` — `go-nflog` listener, extracts IPv4 dst from payload, fans out to channel
- `wol/` — 102-byte magic packet, UDP:9 broadcast, `SO_BINDTODEVICE` + `SO_BROADCAST`
- `main.go` — wiring, processor loop, signal handling (SIGHUP reload, SIGINT/TERM shutdown)

## Key Constraints

- **Root required**: `CAP_NET_ADMIN` + `CAP_NET_RAW` for NFLOG, netlink, `SO_BINDTODEVICE`
- **Go 1.22+** (module declares 1.26)
- **Single NFLOG group**: Only one consumer per group (stop tcpdump first)
- **IPv4 only**: Payload parsing rejects IPv6
- **Rate limiting**: 2-min cache per IP (configurable via `CACHE_TTL`)
- **Static ipset**: `hash:ip,mac` set must be pre-populated; reload via `kill -HUP`

## Configuration (env vars)

| Variable | Required | Default | Notes |
|---|---|---|---|
| `IPSET_NAME` | yes | — | `hash:ip,mac` set name |
| `NFLOG_GROUP` | no | `100` | Must match firewall rule |
| `CACHE_TTL` | no | `2m` | Rate-limit window |
| `TARGET_CHAN_BUF` | no | `64` | Internal channel buffer |

## Firewall Rules

**iptables**:
```bash
iptables -A FORWARD -i tailscale0 -p tcp --syn -j NFLOG \
  --nflog-group 100 --nflog-prefix "TAILSCALE_WOL"
```

**nftables** (adjust ports as needed):
```bash
nft add rule inet filter forward iifname "tailscale0" \
  tcp flags syn tcp dport '{3389,445,22}' \
  log group 100 prefix "TAILSCALE_WOL"
```

## Testing

- Unit tests only (`go test ./...`), no integration tests
- Requires netlink/ipset for `ipset.Refresh()` — tests mock or use local kernel state
- Run with `make test` or `go test -count=1 ./...`

## Common Issues

| Symptom | Check |
|---|---|
| No packets | iptables/nftables counters, `/proc/net/netfilter/nfnetlink_log`, root user |
| IP skipped | `ipset test lan_hosts <ip>`, then SIGHUP |
| WOL fails | `ip route get <ip>` for interface, root for SO_BINDTODEVICE, target BIOS WOL enabled |
| Route lookup fails | Static LAN routes pointing to correct VLAN interfaces |

## Signals

- `SIGHUP` — reload ipset mappings
- `SIGINT` / `SIGTERM` — graceful shutdown

## Dependencies

- `github.com/florianl/go-nflog/v2` — NFLOG
- `github.com/vishvananda/netlink` — route/interface/ipset
- `github.com/mdlayher/netlink` — NFLOG socket options
- `github.com/patrickmn/go-cache` — in-memory rate limiting