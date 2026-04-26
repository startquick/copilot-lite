package httpapi

import (
	"net"
	"net/http"
	"strings"
)

// effectiveClientIP returns the IP used for admin security decisions.
// Proxy headers are only trusted when the immediate peer looks like a local
// reverse proxy hop such as loopback, RFC1918 private space, or link-local.
func effectiveClientIP(r *http.Request) string {
	peerIP := parseIPFromRemoteAddr(r.RemoteAddr)
	if peerIP == nil {
		return r.RemoteAddr
	}
	if !isTrustedProxyPeer(peerIP) {
		return peerIP.String()
	}
	for _, candidate := range forwardedIPs(r) {
		if candidate != nil {
			return candidate.String()
		}
	}
	return peerIP.String()
}

func parseIPFromRemoteAddr(remoteAddr string) net.IP {
	if remoteAddr == "" {
		return nil
	}
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		return net.ParseIP(strings.TrimSpace(remoteAddr))
	}
	return net.ParseIP(strings.TrimSpace(host))
}

func isTrustedProxyPeer(ip net.IP) bool {
	return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast()
}

func forwardedIPs(r *http.Request) []net.IP {
	var ips []net.IP
	for _, part := range strings.Split(r.Header.Get("X-Forwarded-For"), ",") {
		if ip := net.ParseIP(strings.TrimSpace(part)); ip != nil {
			ips = append(ips, ip)
		}
	}
	if ip := net.ParseIP(strings.TrimSpace(r.Header.Get("X-Real-IP"))); ip != nil {
		ips = append(ips, ip)
	}
	return ips
}
