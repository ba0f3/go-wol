package service

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/tui/go-wol/config"
)

const (
	serviceName       = "go-wol"
	unitPath          = "/etc/systemd/system/go-wol.service"
	binaryInstallPath = "/usr/local/bin/go-wol"
)

// Install copies the binary, writes the systemd unit, and enables the service.
func Install(cfg config.Config) error {
	if err := requireRoot(); err != nil {
		return err
	}

	log.Printf("service: installing %s", serviceName)

	if err := installBinary(); err != nil {
		return err
	}

	unit := RenderUnit(cfg, binaryInstallPath)
	if err := os.WriteFile(unitPath, []byte(unit), 0o644); err != nil {
		return fmt.Errorf("write unit file %s: %w", unitPath, err)
	}
	log.Printf("service: wrote %s", unitPath)

	if err := runSystemctl("daemon-reload"); err != nil {
		return err
	}
	if err := runSystemctl("enable", serviceName); err != nil {
		return err
	}
	if err := runSystemctl("start", serviceName); err != nil {
		return err
	}

	log.Printf("service: %s installed and started", serviceName)
	return nil
}

// Uninstall stops, disables, and removes the systemd service.
func Uninstall() error {
	if err := requireRoot(); err != nil {
		return err
	}

	log.Printf("service: uninstalling %s", serviceName)

	if err := runSystemctl("stop", serviceName); err != nil {
		log.Printf("service: stop warning: %v", err)
	}
	if err := runSystemctl("disable", serviceName); err != nil {
		log.Printf("service: disable warning: %v", err)
	}

	if err := os.Remove(unitPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove unit file %s: %w", unitPath, err)
	}
	log.Printf("service: removed %s", unitPath)

	if err := runSystemctl("daemon-reload"); err != nil {
		return err
	}

	if err := os.Remove(binaryInstallPath); err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("remove binary %s: %w", binaryInstallPath, err)
		}
	} else {
		log.Printf("service: removed %s", binaryInstallPath)
	}

	log.Printf("service: %s uninstalled", serviceName)
	return nil
}

// RenderUnit returns the systemd unit file contents for the given config.
func RenderUnit(cfg config.Config, execPath string) string {
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "[Unit]\n")
	fmt.Fprintf(&buf, "Description=Tailscale Wake-on-LAN daemon\n")
	fmt.Fprintf(&buf, "After=network-online.target tailscaled.service\n")
	fmt.Fprintf(&buf, "Wants=network-online.target\n\n")
	fmt.Fprintf(&buf, "[Service]\n")
	fmt.Fprintf(&buf, "Type=simple\n")
	fmt.Fprintf(&buf, "ExecStart=%s\n", execPath)
	fmt.Fprintf(&buf, "Environment=IPSET_NAME=%s\n", cfg.IPSetName)
	fmt.Fprintf(&buf, "Environment=NFLOG_GROUP=%d\n", cfg.NFLogGroup)
	fmt.Fprintf(&buf, "Environment=CACHE_TTL=%s\n", cfg.CacheTTL)
	fmt.Fprintf(&buf, "Environment=TARGET_CHAN_BUF=%d\n", cfg.TargetChanBuf)
	fmt.Fprintf(&buf, "Restart=on-failure\n")
	fmt.Fprintf(&buf, "RestartSec=5\n\n")
	fmt.Fprintf(&buf, "[Install]\n")
	fmt.Fprintf(&buf, "WantedBy=multi-user.target\n")
	return buf.String()
}

func installBinary() error {
	src, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve executable path: %w", err)
	}
	src, err = filepath.EvalSymlinks(src)
	if err != nil {
		return fmt.Errorf("resolve executable symlinks: %w", err)
	}

	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("open source binary %s: %w", src, err)
	}
	defer func() { _ = in.Close() }()

	out, err := os.OpenFile(binaryInstallPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		return fmt.Errorf("create install binary %s: %w", binaryInstallPath, err)
	}

	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		return fmt.Errorf("copy binary to %s: %w", binaryInstallPath, err)
	}
	if err := out.Close(); err != nil {
		return fmt.Errorf("close install binary %s: %w", binaryInstallPath, err)
	}

	log.Printf("service: installed binary %s -> %s", src, binaryInstallPath)
	return nil
}

func requireRoot() error {
	if os.Geteuid() != 0 {
		return fmt.Errorf("must run as root")
	}
	return nil
}

func runSystemctl(args ...string) error {
	cmd := exec.Command("systemctl", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("systemctl %v: %w (output: %s)", args, err, bytes.TrimSpace(out))
	}
	log.Printf("service: systemctl %v", args)
	return nil
}
