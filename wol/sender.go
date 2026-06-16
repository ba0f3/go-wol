package wol

import (
	"fmt"
	"log"
	"net"
	"syscall"
)

// BuildMagicPacket returns the 102-byte WOL magic packet for the given MAC.
func BuildMagicPacket(mac net.HardwareAddr) []byte {
	packet := make([]byte, 102)
	for i := 0; i < 6; i++ {
		packet[i] = 0xff
	}
	for i := 6; i < 102; i += len(mac) {
		copy(packet[i:i+len(mac)], mac)
	}
	return packet
}

// SendMagicPacket crafts a WOL magic packet and sends it via UDP broadcast
// bound to the specified network interface.
func SendMagicPacket(mac string, ifaceName string) error {
	hwAddr, err := net.ParseMAC(mac)
	if err != nil {
		return fmt.Errorf("parse MAC %q: %w", mac, err)
	}
	if len(hwAddr) != 6 {
		return fmt.Errorf("invalid MAC address length %d, expected 6", len(hwAddr))
	}

	packet := BuildMagicPacket(hwAddr)
	log.Printf("wol: sending magic packet for MAC %s via interface %s (%d bytes)",
		mac, ifaceName, len(packet))

	fd, err := syscall.Socket(syscall.AF_INET, syscall.SOCK_DGRAM, 0)
	if err != nil {
		return fmt.Errorf("create UDP socket: %w", err)
	}
	defer func() { _ = syscall.Close(fd) }()

	if err := syscall.SetsockoptString(fd, syscall.SOL_SOCKET, syscall.SO_BINDTODEVICE, ifaceName); err != nil {
		return fmt.Errorf("SO_BINDTODEVICE %q: %w", ifaceName, err)
	}

	if err := syscall.SetsockoptInt(fd, syscall.SOL_SOCKET, syscall.SO_BROADCAST, 1); err != nil {
		return fmt.Errorf("SO_BROADCAST: %w", err)
	}

	addr := syscall.SockaddrInet4{
		Port: 9,
		Addr: [4]byte{255, 255, 255, 255},
	}

	if err := syscall.Sendto(fd, packet, 0, &addr); err != nil {
		return fmt.Errorf("send WOL packet via %s: %w", ifaceName, err)
	}

	log.Printf("wol: successfully sent magic packet for %s via %s", mac, ifaceName)
	return nil
}
