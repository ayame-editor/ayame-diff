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
	fmt.Fprintln(w, "WARNING: remote access is enabled. The GUI can read and write local files, and anyone holding the URL's token can drive it; restrict access with a firewall or trusted network.")
}

// loopbackHosts returns the Host header values a browser will send for a
// loopback listener: the address itself plus the other spellings of the same
// machine. Pinning this set is what stops DNS rebinding, where a page keeps its
// own hostname in Host after making that name resolve to 127.0.0.1 (#108).
func loopbackHosts(addr net.Addr) []string {
	_, port, err := net.SplitHostPort(addr.String())
	if err != nil {
		return nil
	}
	return []string{
		net.JoinHostPort("127.0.0.1", port),
		net.JoinHostPort("localhost", port),
		net.JoinHostPort("::1", port),
	}
}

// tokenURL appends the API token to a base URL, which is how the browser comes
// to hold the credential every API call needs (#108).
func tokenURL(base, token string) string {
	if token == "" {
		return base
	}
	separator := "?"
	if strings.Contains(base, "?") {
		separator = "&"
	}
	return base + separator + url.Values{"token": {token}}.Encode()
}
