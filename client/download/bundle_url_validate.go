package download

import (
	"fmt"
	"net"
	"net/url"
	"strings"
)

func validateBundleURL(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", fmt.Errorf("download: empty url")
	}

	u, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("download: bad url: %w", err)
	}

	if err := validateParsedURL(u); err != nil {
		return "", err
	}

	if err := validateHost(u.Hostname()); err != nil {
		return "", err
	}

	return u.String(), nil
}

func validateParsedURL(u *url.URL) error {
	if !strings.EqualFold(u.Scheme, "https") {
		return fmt.Errorf("download: unsupported url scheme %q", u.Scheme)
	}
	if u.Host == "" {
		return fmt.Errorf("download: url has empty host")
	}
	if u.User != nil {
		return fmt.Errorf("download: url must not contain userinfo")
	}
	if u.Fragment != "" {
		return fmt.Errorf("download: url must not contain fragment")
	}
	return nil
}

func validateHost(host string) error {
	host = strings.ToLower(host)
	if host == "" {
		return fmt.Errorf("download: url has empty hostname")
	}
	if host == "localhost" {
		return fmt.Errorf("download: localhost is not allowed")
	}
	if strings.HasSuffix(host, ".localhost") ||
		strings.HasSuffix(host, ".local") ||
		strings.HasSuffix(host, ".internal") {
		return fmt.Errorf("download: local/internal hostname is not allowed")
	}

	if ip := net.ParseIP(host); ip != nil && isBlockedIP(ip) {
		return fmt.Errorf("download: ip %s is not allowed", ip.String())
	}

	return nil
}

func isBlockedIP(ip net.IP) bool {
	if ip == nil {
		return true
	}
	if v4 := ip.To4(); v4 != nil {
		ip = v4
	} else if v6 := ip.To16(); v6 != nil {
		ip = v6
	} else {
		return true
	}

	// obvious badness
	if ip.IsLoopback() || ip.IsUnspecified() || ip.IsMulticast() {
		return true
	}
	if ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
		return true
	}

	// private ranges (v4 + v6 ULA)
	for _, n := range blockedNets {
		if n.Contains(ip) {
			return true
		}
	}
	return false
}

var blockedNets = []*net.IPNet{
	mustCIDR("10.0.0.0/8"),
	mustCIDR("172.16.0.0/12"),
	mustCIDR("192.168.0.0/16"),
	mustCIDR("127.0.0.0/8"),
	mustCIDR("169.254.0.0/16"), // link-local v4
	mustCIDR("::1/128"),
	mustCIDR("fe80::/10"), // link-local v6
	mustCIDR("fc00::/7"),  // unique local v6
	mustCIDR("::/128"),    // unspecified v6
	mustCIDR("ff00::/8"),  // multicast v6
}

func mustCIDR(s string) *net.IPNet {
	_, n, err := net.ParseCIDR(s)
	if err != nil {
		panic(err)
	}
	return n
}
