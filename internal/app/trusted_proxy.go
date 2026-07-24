package app

import (
	"errors"
	"net"
	"net/http"
	"net/netip"
	"strings"
)

type trustedProxySet []netip.Prefix

func parseTrustedProxyCIDRs(value string) (trustedProxySet, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}
	parts := strings.Split(value, ",")
	result := make(trustedProxySet, 0, len(parts))
	for _, part := range parts {
		prefix, err := netip.ParsePrefix(strings.TrimSpace(part))
		if err != nil {
			return nil, errors.New("ROBLOX_TRUSTED_PROXY_CIDRS contains an invalid CIDR")
		}
		result = append(result, prefix.Masked())
	}
	return result, nil
}

func (set trustedProxySet) contains(address netip.Addr) bool {
	address = address.Unmap()
	for _, prefix := range set {
		if prefix.Contains(address) {
			return true
		}
	}
	return false
}

func remotePeerAddress(r *http.Request) (netip.Addr, bool) {
	remote := strings.TrimSpace(r.RemoteAddr)
	host, _, err := net.SplitHostPort(remote)
	if err != nil {
		host = remote
	}
	address, err := netip.ParseAddr(strings.TrimSpace(host))
	if err != nil {
		return netip.Addr{}, false
	}
	return address.Unmap(), true
}

func requestClientIP(r *http.Request, trusted trustedProxySet) string {
	peer, ok := remotePeerAddress(r)
	if !ok {
		return "unknown"
	}
	if !trusted.contains(peer) {
		return peer.String()
	}
	rawHops := strings.Split(r.Header.Get("X-Forwarded-For"), ",")
	hops := make([]netip.Addr, 0, len(rawHops))
	for _, rawHop := range rawHops {
		hop, err := netip.ParseAddr(strings.TrimSpace(rawHop))
		if err != nil {
			return peer.String()
		}
		hops = append(hops, hop.Unmap())
	}
	if len(hops) == 0 {
		return peer.String()
	}
	for index := len(hops) - 1; index >= 0; index-- {
		if !trusted.contains(hops[index]) {
			return hops[index].String()
		}
	}
	return hops[0].String()
}

func forwardedScheme(r *http.Request, trusted trustedProxySet) string {
	peer, ok := remotePeerAddress(r)
	if !ok || !trusted.contains(peer) {
		return ""
	}
	values := strings.Split(r.Header.Get("X-Forwarded-Proto"), ",")
	if len(values) == 0 {
		return ""
	}
	scheme := strings.ToLower(strings.TrimSpace(values[len(values)-1]))
	if scheme != "http" && scheme != "https" {
		return ""
	}
	return scheme
}
