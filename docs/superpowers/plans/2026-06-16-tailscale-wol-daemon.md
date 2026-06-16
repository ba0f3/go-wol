# Tailscale WOL Daemon Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a Linux daemon that wakes LAN hosts via WOL when Tailscale traffic hits NFLOG.

**Architecture:** Single processor goroutine reads destination IPs from NFLOG via a buffered channel, resolves MAC from ipset, outbound interface from netlink, sends VLAN-bound WOL UDP broadcast. Config via env vars; ipset reload on SIGHUP.

**Tech Stack:** Go 1.22+, go-nflog/v2, vishvananda/netlink, patrickmn/go-cache

---

## File Map

| File | Responsibility |
|---|---|
| `go.mod` | Module + dependencies |
| `config/config.go` | Env-based configuration |
| `config/config_test.go` | Config parsing tests |
| `ipset/resolver.go` | ipset list parse + lookup |
| `ipset/resolver_test.go` | Parser unit tests |
| `network/route.go` | netlink route → interface name |
| `wol/sender.go` | Magic packet craft + SO_BINDTODEVICE send |
| `wol/sender_test.go` | Magic packet byte tests |
| `nflog/listener.go` | NFLOG hook + IPv4 dst extraction |
| `main.go` | Wire-up, signals, processor loop |

## Tasks

- [x] Task 1: Scaffold go.mod and config package
- [x] Task 2: ipset resolver with parser tests
- [x] Task 3: network route lookup
- [x] Task 4: WOL sender with magic packet tests
- [x] Task 5: NFLOG listener
- [x] Task 6: main.go wiring + signal handling
- [x] Task 7: `go build` + `go test ./...`
