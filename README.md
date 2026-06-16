# go-wol

**Linux-only** daemon that wakes LAN hosts via Wake-on-LAN when Tailscale traffic arrives at your router.

**Platform:** Linux only — uses kernel interfaces (NFLOG, ipset, netlink). No macOS/Windows support.

When a remote user connects over Tailscale to a sleeping machine on your LAN, the router sees the forwarded SYN packet. This daemon captures that traffic via NFLOG, looks up the destination host's MAC address from an ipset, determines the correct VLAN interface, and sends a WOL magic packet bound to that interface.

## How it works

```
tailscale0 ──► iptables/nftables (NFLOG) ──► go-wol
                                                │
                    ┌───────────────────────────┼───────────────────────────┐
                    ▼                           ▼                           ▼
              ipset lookup              netlink route               WOL broadcast
              (IP → MAC)                (IP → vlan iface)           (SO_BINDTODEVICE)
```

1. Firewall rules send matching Tailscale forwarded traffic to an NFLOG group.
2. The daemon extracts the IPv4 destination address from each packet.
3. It resolves the MAC from a `hash:ip,mac` ipset.
4. It finds the outbound VLAN interface via `netlink.RouteGet`.
5. It sends a 102-byte magic packet to `255.255.255.255:9` on that interface.
6. A 2-minute rate limiter prevents duplicate WOL sends for the same IP.

## Requirements

- Ubuntu (or any Linux with netfilter NFLOG, ipset, and netlink)
- Go 1.26+ (see `go.mod`)
- Root or `CAP_NET_ADMIN` + `CAP_NET_RAW`
- Tailscale on the router (`tailscale0` interface)
- Static IP-to-MAC mappings in an ipset

## Build

```bash
go build -o go-wol .
```

Run tests:

```bash
go test ./...
```

## Configuration

All settings are loaded from environment variables.

| Variable | Required | Default | Description |
|---|---|---|---|
| `IPSET_NAME` | **yes** | — | Name of the `hash:ip,mac` ipset |
| `NFLOG_GROUP` | no | `100` | NFLOG netlink group ID |
| `CACHE_TTL` | no | `2m` | Rate-limit duration after a successful WOL send |
| `TARGET_CHAN_BUF` | no | `64` | Buffer size for the internal target-IP channel |

Example:

```bash
export IPSET_NAME=lan_hosts
export NFLOG_GROUP=100
export CACHE_TTL=2m

sudo ./go-wol
```

## ipset setup

Create a `hash:ip,mac` set and add your static hosts:

```bash
sudo ipset create lan_hosts hash:ip,mac
sudo ipset add lan_hosts 192.168.10.5,AA:BB:CC:DD:EE:FF
sudo ipset add lan_hosts 192.168.10.6,11:22:33:44:55:66
```

Verify:

```bash
sudo ipset list lan_hosts
```

Reload the in-memory map without restarting the daemon:

```bash
sudo kill -HUP $(pidof go-wol)
```

## Firewall rules

The daemon trusts the firewall to filter traffic. Only packets that match your rules are logged to NFLOG.

> **Note:** The examples below use port **3389 (RDP)** to illustrate waking a Windows PC when a remote desktop connection arrives over Tailscale. go-wol itself only reads the **destination IP** from matched packets — it works for any service. Adjust `--dport` / `tcp dport` in your firewall rules for SSH (22), SMB (445), or omit port filtering to wake on any TCP SYN to LAN hosts.

### iptables

```bash
sudo iptables -A FORWARD -i tailscale0 -p tcp --syn --dport 3389 -j NFLOG \
  --nflog-group 100 --nflog-prefix "TAILSCALE_WOL"
```

### nftables

```bash
sudo nft add rule inet filter forward iifname "tailscale0" \
  tcp flags syn tcp dport '{3389, 445, 22}' \
  log group 100 prefix "TAILSCALE_WOL"
```

Only one process can listen on a given NFLOG group at a time. Stop `tcpdump` or other NFLOG consumers before starting go-wol.

## systemd

> **Note:** The service installer uses hardcoded paths: binary at `/usr/local/bin/go-wol`, unit at `/etc/systemd/system/go-wol.service`. To customize, edit `service/systemd.go` before building, or install manually.

Install the service (copies binary to `/usr/local/bin/go-wol`, writes unit file, enables and starts):

```bash
sudo IPSET_NAME=lan_hosts NFLOG_GROUP=100 CACHE_TTL=2m ./go-wol service install
```

Uninstall:

```bash
sudo ./go-wol service uninstall
```

Reload ipset in the running daemon (sends `SIGHUP`):

```bash
go-wol ipset reload
```

Manage after install:

```bash
sudo systemctl status go-wol
sudo systemctl restart go-wol
go-wol ipset reload              # or: sudo kill -HUP $(pidof go-wol)
```

The generated unit file looks like:

```ini
[Unit]
Description=Tailscale Wake-on-LAN daemon
After=network-online.target tailscaled.service
Wants=network-online.target

[Service]
Type=simple
ExecStart=/usr/local/bin/go-wol
Environment=IPSET_NAME=lan_hosts
Environment=NFLOG_GROUP=100
Environment=CACHE_TTL=2m0s
Environment=TARGET_CHAN_BUF=64
Restart=on-failure
RestartSec=5

[Install]
WantedBy=multi-user.target
```

## Signals

| Signal | Action |
|---|---|
| `SIGHUP` | Reload ipset mappings |
| `SIGINT` / `SIGTERM` | Graceful shutdown |

## Project layout

```
config/     Environment-based configuration
ipset/      ipset lookup via netlink and IP→MAC resolver
service/    systemd install/uninstall helpers
network/    Outbound interface lookup via netlink
nflog/      NFLOG listener and IPv4 header parsing
wol/        Magic packet construction and UDP send
main.go     Wiring, processor loop, signal handling
```

## Dependencies

- [github.com/florianl/go-nflog/v2](https://github.com/florianl/go-nflog) — NFLOG without libpcap
- [github.com/vishvananda/netlink](https://github.com/vishvananda/netlink) — route and interface lookup
- [github.com/patrickmn/go-cache](https://github.com/patrickmn/go-cache) — in-memory rate limiting

## Troubleshooting

**`operation not permitted` on NFLOG bind**

NFLOG requires root or file capabilities:

```bash
sudo ./go-wol
# or
sudo setcap cap_net_admin,cap_net_raw=ep ./go-wol
```

If installed via systemd, the unit runs as root by default (`go-wol service install`).

**No packets received**

- Confirm iptables/nftables counters increment on the NFLOG rule.
- Check that no other process holds the same NFLOG group: `cat /proc/net/netfilter/nfnetlink_log`
- Ensure go-wol runs as root.

**IP skipped (not in ipset)**

- Verify the destination IP exists in the set: `ipset test lan_hosts <ip>`
- Send `SIGHUP` after adding new entries.

**WOL send failed**

- Confirm the VLAN interface name is correct (`ip route get <ip>`).
- Check that `SO_BINDTODEVICE` is permitted (requires root).
- Verify the target machine has WOL enabled in BIOS and the NIC supports magic packets.

**Route lookup failed**

- Ensure a route exists for the target IP on the router.
- Static LAN subnets should have routes pointing to the correct VLAN interface.
