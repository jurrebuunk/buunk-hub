// Package safenet makes HTTP clients for fetching addresses people typed
// (quick links' icons, the sites their pages embed). They only connect to
// public addresses, so nobody signed in can make the hub fetch from inside
// the cluster or the site.
package safenet

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"syscall"
	"time"
)

// Client returns a client that connects only to public addresses, over
// https only (also after redirects), within timeout.
func Client(timeout time.Duration) *http.Client {
	dialer := &net.Dialer{Timeout: timeout, Control: PublicOnly}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.DialContext = dialer.DialContext
	transport.Proxy = nil
	return &http.Client{
		Timeout:   timeout,
		Transport: transport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if req.URL.Scheme != "https" || len(via) >= 5 {
				return errors.New("redirect refused")
			}
			return nil
		},
	}
}

// PublicOnly refuses connections to anything but public addresses. It is
// a net.Dialer Control: checked when connecting, after DNS and on every
// redirect, so a name can't point it elsewhere.
func PublicOnly(_, address string, _ syscall.RawConn) error {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return err
	}
	ip, err := netip.ParseAddr(host)
	if err != nil {
		return err
	}
	ip = ip.Unmap()
	if !ip.IsGlobalUnicast() || ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() ||
		cgnat.Contains(ip) || ip.IsMulticast() || ip.IsUnspecified() {
		return fmt.Errorf("%s is not a public address", ip)
	}
	return nil
}

// 100.64.0.0/10, shared address space (carrier-grade NAT, Tailscale).
var cgnat = netip.MustParsePrefix("100.64.0.0/10")
