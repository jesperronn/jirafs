// Package netguard provides a RoundTripper that refuses non-loopback requests.
// It is installed only under tests so they can never reach the network.
package netguard

import (
	"fmt"
	"net"
	"net/http"
	"strings"
)

// IsLoopbackHost reports whether host (optionally with port) is loopback.
func IsLoopbackHost(host string) bool {
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	host = strings.Trim(host, "[]")
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

type guard struct{ next http.RoundTripper }

// Wrap returns a RoundTripper refusing any request to a non-loopback host.
func Wrap(next http.RoundTripper) http.RoundTripper {
	if next == nil {
		next = http.DefaultTransport
	}
	return guard{next: next}
}

func (g guard) RoundTrip(req *http.Request) (*http.Response, error) {
	if !IsLoopbackHost(req.URL.Host) {
		if req.Body != nil {
			req.Body.Close()
		}
		return nil, fmt.Errorf("netguard: tests may only make loopback requests; refused %s", req.URL.Redacted())
	}
	return g.next.RoundTrip(req)
}
