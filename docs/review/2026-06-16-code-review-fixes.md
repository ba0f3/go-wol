# go-wol Code Review: Findings & Fixes

**Date:** 2026-06-16
**Review:** Automated code review via Sisyphus subagent (deep category)
**Scope:** All Go source files — `main.go`, `config/config.go`, `ipset/resolver.go`,
`network/route.go`, `nflog/listener.go`, `wol/sender.go`, plus unit tests
**Target:** Bugs, memory leaks, security vulnerabilities

---

## Found: 6 issues (2 Critical, 2 Important, 2 Minor)

### Critical

| # | Issue | Severity | What Happens |
|---|-------|----------|--------------|
| C1 | **Shutdown race — panic on close** | Crash | `close(targetIPs)` races with `go-nflog` hook goroutine still writing to channel. `send on closed channel` panic during graceful shutdown. |
| C2 | **Zombie daemon on listener failure** | Hang | If `listener.Run(ctx)` exits with error (e.g., socket breaks), the goroutine dies silently. Main loop blocks forever on `for sig := range sigCh`. Daemon stops processing but never exits — systemd won't restart it. |

### Important

| # | Issue | Severity | What Happens |
|---|-------|----------|--------------|
| I1 | **Infinite loop on socket error** | 100% CPU spin | `errFunc` always returns 0, telling `go-nflog` to retry. Persistent errors (closed socket, `ENOBUFS`) cause tight loop logging and consuming CPU forever. |
| I2 | **Panic on non-6-byte MAC** | Crash | `BuildMagicPacket` assumes MAC is exactly 6 bytes. If `len(mac) != 6`, the copy loop panics: `slice bounds out of range` on fixed 102-byte buffer. |

### Minor

| # | Issue | Severity | What Happens |
|---|-------|----------|--------------|
| M1 | **Unbounded channel from env** | Memory | `TARGET_CHAN_BUF` parsed from env with no upper bound. Extremely large value causes OOM on channel allocation. |
| M2 | **Missing nil check** | Panic | `entriesFromResult` dereferences `result` without nil check. If `netlink.IpsetList` ever returns nil+no error, panic. |

---

## Fixed: How Each Fix Was Applied

### C1 — Shutdown race (main.go)

**Root cause:** Shutdown sequence was: `cancel()` → `close(targetIPs)` → `wg.Wait()`. The `go-nflog` library's internal goroutine may still execute the packet hook after `cancel()` returns, attempting to send on a closed channel.

**Fix (main.go:115, 151-178, 180-228):**

1. **Eliminated `close(targetIPs)` entirely** — channel is garbage-collected when unreferenced.
2. **Passed `context.Context` to `processTargets`** — processor selects on both `ctx.Done()` and `targets` channel, exiting cleanly when context is cancelled.
3. **Processor selects with `ctx.Done()`** — replaced raw `for ip := range targets` with `for { select { case <-ctx.Done(): return; case ip, ok := <-targets: ... } }`. Handles channel close explicitly with `!ok`.

```go
// Before (shutdown path in processTargets):
for ip := range targets {
    // processing...
}

// After:
for {
    select {
    case <-ctx.Done():
        return
    case ip, ok := <-targets:
        if !ok {
            return
        }
        // processing...
    }
}
```

### C2 — Zombie daemon (main.go)

**Root cause:** Main goroutine blocked on `for sig := range sigCh` with no escape for goroutine errors. A fatal listener error left the process alive but non-functional.

**Fix (main.go:128-179):**

1. **Added error channel `errCh := make(chan error, 2)`** — listener goroutine sends fatal errors here.
2. **Listener goroutine sends to errCh** — if `listener.Run()` returns a non-canceled error, it pushes to `errCh` with non-blocking select.
3. **Main loop uses `select` over 3 channels** — `sigCh`, `errCh`, `ctx.Done()`. Any one of them triggers shutdown.
4. **Added fallback `case <-ctx.Done()`** — catches unexpected cancellation as safety net.

```go
// Before:
for sig := range sigCh {
    switch sig {
    case syscall.SIGHUP: // reload
    case syscall.SIGINT, syscall.SIGTERM:
        cancel()
        close(targetIPs)
        wg.Wait()
        return
    }
}

// After:
for {
    select {
    case sig := <-sigCh:
        switch sig {
        case syscall.SIGHUP: // reload
        case syscall.SIGINT, syscall.SIGTERM:
            cancel()
            wg.Wait()
            return
        }
    case err := <-errCh:
        cancel()
        wg.Wait()
        return
    case <-ctx.Done():
        cancel()
        wg.Wait()
        return
    }
}
```

### I1 — Infinite loop on socket error (nflog/listener.go)

**Root cause:** `go-nflog` treats return value 0 from `errFunc` as "continue receiving." Persistent errors cause infinite retry + log spam.

**Fix (nflog/listener.go:66-72):**

Check if the error implements the `Temporary()` interface. Return `0` (continue) only for temporary errors, `1` (abort) for fatal errors.

```go
// Before:
errFunc := func(err error) int {
    log.Printf("nflog: hook error: %v", err)
    return 0 // always retry — infinite loop on persistent errors
}

// After:
errFunc := func(err error) int {
    log.Printf("nflog: hook error: %v", err)
    if nlerr, ok := err.(interface{ Temporary() bool }); ok && nlerr.Temporary() {
        return 0 // retry on temporary errors
    }
    return 1 // abort on fatal errors
}
```

### I2 — Panic on invalid MAC length (wol/sender.go)

**Root cause:** `BuildMagicPacket` copies MAC into a 102-byte buffer with stride `len(mac)`. If MAC is not 6 bytes (e.g., 20-byte HW address), `copy(packet[i:i+len(mac)], mac)` accesses out-of-bounds.

**Fix (wol/sender.go:29-31):**

Validate MAC length in `SendMagicPacket` *after* parsing, *before* building packet.

```go
// Added between ParseMAC and BuildMagicPacket:
if len(hwAddr) != 6 {
    return fmt.Errorf("invalid MAC address length %d, expected 6", len(hwAddr))
}
```

### M1 — Unbounded channel size (config/config.go)

**Root cause:** `TARGET_CHAN_BUF` accepted directly from env without cap.

**Fix (config/config.go:11, 55-57):**

```go
const maxTargetChanBuf = 10000

// In LoadFromEnv, after parsing:
if n > maxTargetChanBuf {
    return Config{}, fmt.Errorf("TARGET_CHAN_BUF %d exceeds maximum %d", n, maxTargetChanBuf)
}
```

### M2 — Missing nil check (ipset/resolver.go)

**Root cause:** `entriesFromResult` dereferenced `result.Entries` without nil check.

**Fix (ipset/resolver.go:55-57):**

```go
// Added at top of entriesFromResult:
if result == nil {
    return nil
}
```

---

## Verification

| Check | Result |
|-------|--------|
| `go vet ./...` | No issues |
| `go test -count=1 ./...` | 8 tests passed |
| `go build -o /dev/null .` | Success |

## Files Changed

| File | Lines Changed | Issues |
|------|--------------|--------|
| `main.go` | ~30 lines | C1, C2 — shutdown race + zombie daemon |
| `nflog/listener.go` | ~6 lines | I1 — infinite loop on socket error |
| `wol/sender.go` | ~3 lines | I2 — invalid MAC panic |
| `config/config.go` | ~5 lines | M1 — unbounded channel size |
| `ipset/resolver.go` | ~3 lines | M2 — missing nil check |