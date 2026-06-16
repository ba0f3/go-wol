package service

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
)

// ReloadIPSet signals the running daemon to reload ipset mappings from the kernel.
func ReloadIPSet() error {
	pid, err := daemonPID()
	if err != nil {
		return err
	}

	if err := syscall.Kill(pid, syscall.SIGHUP); err != nil {
		return fmt.Errorf("send SIGHUP to pid %d: %w", pid, err)
	}

	log.Printf("service: sent SIGHUP to go-wol (pid %d) to reload ipset", pid)
	return nil
}

func daemonPID() (int, error) {
	if pid, err := systemdMainPID(); err == nil {
		return pid, nil
	}

	self := os.Getpid()
	return pidofDaemon(self)
}

func systemdMainPID() (int, error) {
	out, err := exec.Command("systemctl", "show", "-p", "MainPID", "--value", serviceName).Output()
	if err != nil {
		return 0, err
	}

	pid, err := strconv.Atoi(strings.TrimSpace(string(out)))
	if err != nil || pid <= 0 {
		return 0, fmt.Errorf("go-wol service is not running")
	}

	return pid, nil
}

func pidofDaemon(selfPID int) (int, error) {
	out, err := exec.Command("pidof", serviceName).Output()
	if err != nil {
		return 0, fmt.Errorf("go-wol daemon not running")
	}

	pids, err := parsePIDList(string(out))
	if err != nil {
		return 0, err
	}

	var daemonPID int
	for _, pid := range pids {
		if pid == selfPID {
			continue
		}
		if daemonPID != 0 {
			return 0, fmt.Errorf("multiple go-wol processes found: %v", pids)
		}
		daemonPID = pid
	}

	if daemonPID == 0 {
		return 0, fmt.Errorf("go-wol daemon not running")
	}

	return daemonPID, nil
}

func parsePIDList(output string) ([]int, error) {
	fields := strings.Fields(strings.TrimSpace(output))
	if len(fields) == 0 {
		return nil, fmt.Errorf("no pids in output")
	}

	pids := make([]int, 0, len(fields))
	for _, field := range fields {
		pid, err := strconv.Atoi(field)
		if err != nil {
			return nil, fmt.Errorf("parse pid %q: %w", field, err)
		}
		pids = append(pids, pid)
	}

	return pids, nil
}
