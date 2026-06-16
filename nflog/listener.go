package nflog

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"strings"
	"syscall"

	nflog "github.com/florianl/go-nflog/v2"
	"github.com/mdlayher/netlink"
	"github.com/tui/go-wol/privileges"
)

// Listener receives NFLOG packets and forwards destination IPv4 addresses.
type Listener struct {
	nf      *nflog.Nflog
	targets chan<- string
}

// NewListener opens an NFLOG socket for the given group.
func NewListener(group uint16, targets chan<- string) (*Listener, error) {
	cfg := nflog.Config{
		Group:    group,
		Copymode: nflog.CopyPacket,
	}

	nf, err := nflog.Open(&cfg)
	if err != nil {
		return nil, fmt.Errorf("open nflog group %d: %w", group, err)
	}

	if err := nf.Con.SetReadBuffer(512 * 1024); err != nil {
		log.Printf("nflog: warning: failed to set read buffer: %v", err)
	}

	if err := nf.SetOption(netlink.NoENOBUFS, true); err != nil {
		log.Printf("nflog: warning: failed to set NoENOBUFS: %v", err)
	}

	log.Printf("nflog: listening on group %d", group)
	return &Listener{nf: nf, targets: targets}, nil
}

// Run registers the packet hook and blocks until ctx is cancelled.
func (l *Listener) Run(ctx context.Context) error {
	hook := func(attrs nflog.Attribute) int {
		if attrs.Payload == nil {
			return 0
		}

		dstIP, ok := extractIPv4Destination(*attrs.Payload)
		if !ok {
			return 0
		}

		log.Printf("nflog: captured packet with destination %s", dstIP)

		select {
		case l.targets <- dstIP:
		default:
			log.Printf("nflog: target channel full, dropping IP %s", dstIP)
		}

		return 0
	}

	errFunc := func(err error) int {
		log.Printf("nflog: hook error: %v", err)
		if nlerr, ok := err.(interface{ Temporary() bool }); ok && nlerr.Temporary() {
			return 0 // retry on temporary errors
		}
		return 1 // abort on fatal errors
	}

	if err := l.nf.RegisterWithErrorFunc(ctx, hook, errFunc); err != nil {
		return fmt.Errorf("register nflog hook: %w", wrapNFLogError(err))
	}

	<-ctx.Done()
	log.Printf("nflog: context cancelled, stopping listener")
	return ctx.Err()
}

// Close closes the underlying NFLOG socket.
func (l *Listener) Close() error {
	if l.nf == nil {
		return nil
	}
	log.Printf("nflog: closing socket")
	return l.nf.Close()
}

func extractIPv4Destination(payload []byte) (string, bool) {
	if len(payload) < 20 {
		return "", false
	}

	version := payload[0] >> 4
	if version != 4 {
		return "", false
	}

	ihl := int(payload[0]&0x0f) * 4
	if ihl < 20 || len(payload) < ihl {
		return "", false
	}

	dst := net.IPv4(payload[16], payload[17], payload[18], payload[19])
	return dst.String(), true
}

func wrapNFLogError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, syscall.EPERM) || strings.Contains(err.Error(), "not permitted") {
		return fmt.Errorf("%w\n%w", err, privileges.NetfilterError())
	}
	return err
}
