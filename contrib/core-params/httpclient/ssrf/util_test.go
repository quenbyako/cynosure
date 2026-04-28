package ssrf //nolint:testpackage // utility file for tests

import (
	"net/netip"
)

func BuildGuardian(
	networks []string,
	ports []uint16,
	allowedv4Prefixes []netip.Prefix,
	allowedv6Prefixes []netip.Prefix,
	deniedv4Prefixes []netip.Prefix,
	deniedv6Prefixes []netip.Prefix,
) *Guardian {
	return &Guardian{
		networks:          networks,
		ports:             ports,
		allowedv4Prefixes: allowedv4Prefixes,
		allowedv6Prefixes: allowedv6Prefixes,
		deniedv4Prefixes:  deniedv4Prefixes,
		deniedv6Prefixes:  deniedv6Prefixes,
	}
}
