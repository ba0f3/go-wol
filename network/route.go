package network

import (
	"fmt"
	"log"
	"net"

	"github.com/vishvananda/netlink"
)

// GetInterfaceForIP returns the outbound interface name used to reach ip.
func GetInterfaceForIP(ip string) (string, error) {
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return "", fmt.Errorf("invalid IP address %q", ip)
	}

	log.Printf("network: looking up route for %s", ip)

	routes, err := netlink.RouteGet(parsed)
	if err != nil {
		return "", fmt.Errorf("route lookup for %s: %w", ip, err)
	}
	if len(routes) == 0 {
		return "", fmt.Errorf("no route found for %s", ip)
	}

	route := routes[0]
	log.Printf("network: selected route dst=%s gw=%s oif=%d for %s",
		route.Dst, route.Gw, route.LinkIndex, ip)

	link, err := netlink.LinkByIndex(route.LinkIndex)
	if err != nil {
		return "", fmt.Errorf("link lookup for index %d: %w", route.LinkIndex, err)
	}

	ifaceName := link.Attrs().Name
	log.Printf("network: interface for %s is %s", ip, ifaceName)
	return ifaceName, nil
}
