package main

import (
	"fmt"
	"io"
	"net"
	"net/url"
	"strings"
)

// remoteBind reports whether addr can accept non-loopback traffic. Hostnames
// other than the special localhost name require opt-in: accepting a DNS answer
// here would let its meaning change between validation and Listen.
func remoteBind(addr string) (bool, error) {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return false, fmt.Errorf("invalid listen address %q: %w", addr, err)
	}
	if strings.EqualFold(host, "localhost") {
		return false, nil
	}
	ip := net.ParseIP(host)
	return ip == nil || !ip.IsLoopback(), nil
}

// browserBaseURL turns a listener address into a URL a browser can actually
// open. Wildcard addresses are valid bind targets but not useful destinations,
// so use the corresponding loopback address while preserving the chosen port.
func browserBaseURL(addr net.Addr) string {
	host, port, err := net.SplitHostPort(addr.String())
	if err != nil {
		return "http://" + addr.String() + "/"
	}
	if ip := net.ParseIP(host); ip != nil && ip.IsUnspecified() {
		if ip.To4() != nil {
			host = "127.0.0.1"
		} else {
			host = "::1"
		}
	}
	return (&url.URL{
		Scheme: "http",
		Host:   net.JoinHostPort(host, port),
		Path:   "/",
	}).String()
}

func printRemoteWarning(w io.Writer) {
	fmt.Fprintln(w, "WARNING: remote access is enabled. The GUI can read and write local files and has no authentication; restrict access with a firewall or trusted network.")
}
