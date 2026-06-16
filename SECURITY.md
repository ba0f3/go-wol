# Security Policy

## Supported Versions

Only the latest release is supported with security updates.

| Version | Supported |
|---------|-----------|
| Latest  | ✅        |
| Older   | ❌        |

## Reporting a Vulnerability

**Please do not report security vulnerabilities through public GitHub issues.**

Instead, email security@[your-domain].com with details. You'll receive an acknowledgment within 48 hours.

If the issue is confirmed, we'll work on a fix and coordinate disclosure.

## Security Model

go-wol is a Linux daemon that:
- Runs as root (requires `CAP_NET_ADMIN` + `CAP_NET_RAW`)
- Listens on NFLOG netlink group (no network ports opened)
- Reads kernel ipset via netlink
- Sends UDP broadcast on port 9 via raw socket with `SO_BINDTODEVICE`

### Attack Surface
- **No listening ports** — cannot be reached remotely
- **No shell execution** — no command injection vectors
- **No file I/O** beyond config/env — no path traversal
- **Packet parsing** — validates bounds, rejects IPv6, malformed headers

### Threat Considerations
- **IP spoofing** — firewall (iptables/nftables) must filter to `tailscale0` SYN traffic
- **Rate limiting** — 2-min cache per IP prevents duplicate WOL sends
- **Privilege escalation** — runs as root; ensure binary is immutable (`chattr +i`)

## Hardening Recommendations

1. **Run with minimal caps** instead of root:
   ```bash
   setcap cap_net_admin,cap_net_raw=ep /usr/local/bin/go-wol
   sudo -u nobody ./go-wol
   ```

2. **Restrict NFLOG group** to a unique, high-numbered group

3. **Audit ipset** entries regularly — only trusted MACs

4. **Monitor logs** for:
   - "nflog: hook error" (socket issues)
   - "WOL send failed" (network problems)
   - "route lookup failed" (config issues)