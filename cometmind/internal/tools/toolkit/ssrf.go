package toolkit

import (
	"fmt"
	"net"
	"strings"
)

// GuardAgainstSSRF rejects hostnames that resolve to loopback, private, or
// link-local addresses (cloud metadata, the local cometmind server, etc.).
func GuardAgainstSSRF(host string) error {
	if host == "" {
		return fmt.Errorf("missing host")
	}
	lower := strings.ToLower(host)
	if lower == "localhost" {
		return fmt.Errorf("refusing to fetch a local address: %s", host)
	}

	// If the host is a literal IP, check it directly; otherwise resolve it.
	var ips []net.IP
	if ip := net.ParseIP(host); ip != nil {
		ips = []net.IP{ip}
	} else {
		resolved, err := net.LookupIP(host)
		if err != nil {
			return fmt.Errorf("could not resolve host: %s", host)
		}
		ips = resolved
	}
	for _, ip := range ips {
		if isBlockedIP(ip) {
			return fmt.Errorf("refusing to fetch a private or local address: %s", host)
		}
	}
	return nil
}

func isBlockedIP(ip net.IP) bool {
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() || ip.IsUnspecified() {
		return true
	}
	// Block the IPv4 cloud-metadata address explicitly (covered by link-local,
	// but make the intent clear).
	if ip4 := ip.To4(); ip4 != nil && ip4[0] == 169 && ip4[1] == 254 {
		return true
	}
	return false
}
