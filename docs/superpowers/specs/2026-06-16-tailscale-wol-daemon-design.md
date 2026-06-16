# Tailscale WOL Daemon — Design Spec

**Date:** 2026-06-16  
**Status:** Approved and implemented  
**Module:** `github.com/tui/go-wol`

## Overview

A Linux daemon that listens to NFLOG for forwarded Tailscale traffic, resolves the destination host's MAC address from an ipset (`hash:ip,mac`), determines the outbound VLAN interface via netlink routing, and sends a Wake-on-LAN magic packet bound to that interface. Rate limiting prevents duplicate WOL sends for the same IP within 2 minutes.

## Decisions Summary

| Topic | Decision |
|---|---|
| Config loading | Environment variables only |
| IPSet reload | Startup + `SIGHUP` (no periodic polling) |
| WOL send failure | Do not cache; retry on next NFLOG event |
| Packet filtering | Trust iptables/nftables; userspace only parses IPv4 dst |
| Concurrency | Single processor goroutine + buffered channel |
| Logging | `log.Printf` throughout |

## Architecture

```
┌─────────────┐     iptables/nftables      ┌──────────────┐
│ tailscale0  │ ──► NFLOG group N ────────►│ nflog pkg    │
│  (FORWARD)  │                            │  listener    │
└─────────────┘                            └──────┬───────┘
                                                  │ dst IP
                                                  ▼
                                           ┌──────────────┐
                                           │ targetIPs    │
                                           │ chan (buf 64)│
                                           └──────┬───────┘
                                                  │
                                                  ▼
                                    ┌─────────────────────────┐
                                    │ processor goroutine     │
                                    │ 1. cache.Get(ip) → skip │
                                    │ 2. ipset.GetMac(ip)     │
                                    │ 3. route.GetIface(ip)   │
                                    │ 4. wol.SendMagicPacket  │
                                    │ 5. cache.Set (success)  │
                                    └─────────────────────────┘
```

## Environment Variables

| Variable | Required | Default | Description |
|---|---|---|---|
| `NFLOG_GROUP` | no | `100` | NFLOG netlink group ID |
| `IPSET_NAME` | **yes** | — | Name of `hash:ip,mac` ipset |
| `CACHE_TTL` | no | `2m` | Rate-limit duration after successful WOL |
| `TARGET_CHAN_BUF` | no | `64` | Buffer size for target IP channel |

All durations use Go's `time.ParseDuration` format (e.g. `2m`, `30s`).

## Package Design

### `config/config.go`

```go
type Config struct {
    NFLogGroup   uint16
    IPSetName    string
    CacheTTL     time.Duration
    TargetChanBuf int
}

func LoadFromEnv() (Config, error)
```

- `LoadFromEnv()` reads env vars, validates required fields, returns error on missing `IPSET_NAME` or invalid duration.
- Logs loaded config at startup.

### `ipset/resolver.go`

```go
type Resolver struct {
    setName string
    mu      sync.RWMutex
    entries map[string]string // IP → MAC
}

func NewResolver(setName string) *Resolver
func (r *Resolver) Refresh() error   // exec: ipset list <name>
func (r *Resolver) GetMac(ip string) (string, bool)
```

- `Refresh()` runs `ipset list <set_name>`, parses lines matching `^\s*(\d+\.\d+\.\d+\.\d+),([0-9A-Fa-f:]{17})$`.
- Rebuilds map under write lock, swaps atomically (new map, replace pointer).
- Logs entry count on refresh.
- Called at startup and on `SIGHUP`.

### `network/route.go`

```go
func GetInterfaceForIP(ip string) (string, error)
```

- Parses IP with `net.ParseIP`.
- Calls `netlink.RouteGet(parsedIP)`.
- Returns `LinkAttrs.Name` from the first route's `LinkIndex` via `netlink.LinkByIndex`.
- Logs route and selected interface.
- Returns error if no routes or link lookup fails.

### `wol/sender.go`

```go
func SendMagicPacket(mac string, ifaceName string) error
```

- Parses MAC with `net.ParseMAC`.
- Builds 102-byte packet: 6× `0xFF` + 16× MAC bytes.
- Creates UDP socket to `255.255.255.255:9`.
- Sets `SO_BINDTODEVICE` to `ifaceName` via `syscall.SetsockoptString`.
- Sends packet, closes socket.
- Logs MAC, interface, and byte count.

### `nflog/listener.go`

```go
type Listener struct {
    // wraps go-nflog
}

func NewListener(group uint16, targets chan<- string) (*Listener, error)
func (l *Listener) Run(ctx context.Context) error
func (l *Listener) Close() error
```

- Registers NFLOG callback for configured group.
- On each packet: extract IPv4 destination from payload using `gopacket` or manual header parse (minimum 20 bytes, version 4).
- Non-IPv4 or unparseable packets: log and skip.
- Sends `dstIP.String()` to `targets` channel (non-blocking; log if channel full).
- `Run` blocks until context cancelled, then closes NFLOG handle.

### `main.go`

Startup sequence:

1. `config.LoadFromEnv()`
2. Create `go-cache` with `CACHE_TTL` default expiration, no cleanup interval needed (cache handles internally)
3. `ipset.NewResolver` + initial `Refresh()`
4. Create buffered `targetIPs` channel
5. Start NFLOG listener goroutine
6. Start processor goroutine
7. Register signal handler: `SIGINT`, `SIGTERM` → cancel context; `SIGHUP` → `ipset.Refresh()`

Processor loop (single goroutine):

```
for ip := range targetIPs:
    if cache.Get(ip) found → log skip, continue
    mac, ok := ipset.GetMac(ip); if !ok → log skip, continue
    iface, err := network.GetInterfaceForIP(ip); if err → log error, continue
    err = wol.SendMagicPacket(mac, iface); if err → log error, continue (no cache)
    cache.Set(ip, true, CACHE_TTL); log success
```

Shutdown:

- Cancel context → NFLOG listener stops → close `targetIPs` channel → processor exits.
- Wait for goroutines via `sync.WaitGroup`.

## Dependencies

```
github.com/florianl/go-nflog/v2
github.com/vishvananda/netlink
github.com/patrickmn/go-cache
```

Standard library: `os/exec`, `syscall`, `net`, `log`, `os/signal`, `context`, `sync`.

## Error Handling

| Scenario | Behavior |
|---|---|
| IP not in ipset | Log, skip |
| No route to IP | Log error, skip |
| WOL send fails | Log error, skip (no cache) |
| NFLOG non-IPv4 | Log, skip |
| Channel full | Log warning, drop IP |
| `ipset list` fails on SIGHUP | Log error, keep stale map |
| Missing env var | Fatal at startup |

## Deployment

### Privileges

Daemon must run as root (or with `CAP_NET_ADMIN` + `CAP_NET_RAW`) for:

- NFLOG netlink socket
- `SO_BINDTODEVICE`
- `ipset list` (read-only, usually no special cap)
- `netlink.RouteGet` (read routing table)

### iptables (group 100)

Examples use port **3389 (RDP)** — waking a PC when a remote desktop connection arrives. go-wol only needs the packet's destination IP, so the same setup works for SSH, SMB, or any other service; change or remove the port filter in firewall rules as needed.

```bash
iptables -A FORWARD -i tailscale0 -p tcp --syn --dport 3389 -j NFLOG \
  --nflog-group 100 --nflog-prefix "TAILSCALE_WOL"
```

### nftables (group 100)

```bash
nft add rule inet filter forward iifname "tailscale0" \
  tcp flags syn tcp dport '{3389, 445, 22}' \
  log group 100 prefix "TAILSCALE_WOL"
```

### systemd example

```ini
[Service]
Environment=IPSET_NAME=lan_hosts
Environment=NFLOG_GROUP=100
Environment=CACHE_TTL=2m
ExecStart=/usr/local/bin/go-wol
Restart=on-failure
```

## Testing Strategy

| Package | Test approach |
|---|---|
| `ipset` | Unit test parser with fixture `ipset list` output |
| `wol` | Unit test magic packet byte construction |
| `network` | Integration test (skip if no routes) |
| `nflog` | Manual/integration only (requires NFLOG traffic) |
| `config` | Unit test env parsing |

## Out of Scope

- IPv6 support
- CLI flags or config files
- Periodic ipset polling
- systemd unit file in repo (documented only)
- Userspace port/protocol filtering
