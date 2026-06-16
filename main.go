package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/patrickmn/go-cache"
	"github.com/tui/go-wol/config"
	"github.com/tui/go-wol/ipset"
	"github.com/tui/go-wol/network"
	"github.com/tui/go-wol/nflog"
	"github.com/tui/go-wol/service"
	"github.com/tui/go-wol/wol"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)

	if len(os.Args) > 1 {
		if err := handleCommand(os.Args[1:]); err != nil {
			log.Fatalf("go-wol: %v", err)
		}
		return
	}

	runDaemon()
}

func handleCommand(args []string) error {
	switch args[0] {
	case "service":
		return handleServiceCommand(args[1:])
	case "ipset":
		return handleIPSetCommand(args[1:])
	case "help", "-h", "--help":
		printUsage()
		return nil
	default:
		return fmt.Errorf("unknown command %q (try: go-wol help)", args[0])
	}
}

func handleServiceCommand(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: go-wol service <install|uninstall>")
	}

	switch args[0] {
	case "install":
		cfg, err := config.LoadFromEnv()
		if err != nil {
			return err
		}
		return service.Install(cfg)
	case "uninstall":
		return service.Uninstall()
	default:
		return fmt.Errorf("usage: go-wol service <install|uninstall>")
	}
}

func handleIPSetCommand(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: go-wol ipset <reload>")
	}

	switch args[0] {
	case "reload":
		return service.ReloadIPSet()
	default:
		return fmt.Errorf("usage: go-wol ipset <reload>")
	}
}

func printUsage() {
	fmt.Println(`go-wol - Tailscale Wake-on-LAN daemon

Usage:
  go-wol                          Run the daemon (requires env config)
  go-wol service install          Install and start systemd service (root)
  go-wol service uninstall        Stop and remove systemd service (root)
  go-wol ipset reload             Reload ipset mappings in running daemon
  go-wol help                     Show this help

Environment variables:
  IPSET_NAME        (required) ipset hash:ip,mac set name
  NFLOG_GROUP       NFLOG group ID (default: 100)
  CACHE_TTL         WOL rate-limit duration (default: 2m)
  TARGET_CHAN_BUF   Internal channel buffer (default: 64)

Install example:
  sudo IPSET_NAME=lan_hosts ./go-wol service install`)
}

func runDaemon() {
	log.Printf("go-wol: starting")

	cfg, err := config.LoadFromEnv()
	if err != nil {
		log.Fatalf("go-wol: config error: %v", err)
	}

	resolver := ipset.NewResolver(cfg.IPSetName)
	if err := resolver.Refresh(); err != nil {
		log.Fatalf("go-wol: initial ipset refresh failed: %v", err)
	}

	rateLimit := cache.New(cfg.CacheTTL, cfg.CacheTTL)
	targetIPs := make(chan string, cfg.TargetChanBuf)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	listener, err := nflog.NewListener(cfg.NFLogGroup, targetIPs)
	if err != nil {
		log.Fatalf("go-wol: nflog listener error: %v", err)
	}
	defer func() { _ = listener.Close() }()

	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := listener.Run(ctx); err != nil && err != context.Canceled {
			log.Printf("go-wol: nflog listener exited: %v", err)
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		processTargets(targetIPs, resolver, rateLimit, cfg.CacheTTL)
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)

	for sig := range sigCh {
		switch sig {
		case syscall.SIGHUP:
			log.Printf("go-wol: received SIGHUP, reloading ipset")
			if err := resolver.Refresh(); err != nil {
				log.Printf("go-wol: ipset refresh failed: %v", err)
			}
		case syscall.SIGINT, syscall.SIGTERM:
			log.Printf("go-wol: received %s, shutting down", sig)
			cancel()
			close(targetIPs)
			wg.Wait()
			log.Printf("go-wol: shutdown complete")
			return
		}
	}
}

func processTargets(
	targets <-chan string,
	resolver *ipset.Resolver,
	rateLimit *cache.Cache,
	cacheTTL time.Duration,
) {
	for ip := range targets {
		log.Printf("processor: handling target IP %s", ip)

		if _, found := rateLimit.Get(ip); found {
			log.Printf("processor: skipping %s (WOL sent recently)", ip)
			continue
		}

		mac, ok := resolver.GetMac(ip)
		if !ok {
			log.Printf("processor: skipping %s (not in ipset)", ip)
			continue
		}
		log.Printf("processor: resolved %s -> MAC %s", ip, mac)

		iface, err := network.GetInterfaceForIP(ip)
		if err != nil {
			log.Printf("processor: route lookup failed for %s: %v", ip, err)
			continue
		}

		if err := wol.SendMagicPacket(mac, iface); err != nil {
			log.Printf("processor: WOL send failed for %s: %v", ip, err)
			continue
		}

		rateLimit.Set(ip, true, cacheTTL)
		log.Printf("processor: WOL sent for %s, cached for %s", ip, cacheTTL)
	}

	log.Printf("processor: channel closed, exiting")
}
