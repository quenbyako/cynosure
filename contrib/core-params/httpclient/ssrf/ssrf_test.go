package ssrf_test

import (
	"errors"
	"fmt"
	"net/netip"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	. "github.com/quenbyako/cynosure/contrib/core-params/httpclient/ssrf"
)

func TestOptions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		Name    string
		Options []Option
		Result  *Guardian
	}{{
		Name:    "default",
		Options: nil,
		Result: BuildGuardian(
			[]string{"tcp4", "tcp6"}, []uint16{80, 443},
			nil, nil,
			IPv4DeniedPrefixes, IPv6DeniedPrefixes,
		),
	}, {
		Name:    "with port 53",
		Options: []Option{WithPorts(53)},
		Result: BuildGuardian(
			[]string{"tcp4", "tcp6"}, []uint16{53},
			nil, nil,
			IPv4DeniedPrefixes, IPv6DeniedPrefixes,
		),
	}, {
		Name:    "with multiple port calls",
		Options: []Option{WithPorts(52), WithPorts(53)},
		Result: BuildGuardian(
			[]string{"tcp4", "tcp6"}, []uint16{53},
			nil, nil,
			IPv4DeniedPrefixes, IPv6DeniedPrefixes,
		),
	}, {
		Name:    "with port without argument",
		Options: []Option{WithPorts()},
		Result: BuildGuardian(
			[]string{"tcp4", "tcp6"}, nil,
			nil, nil,
			IPv4DeniedPrefixes, IPv6DeniedPrefixes,
		),
	}, {
		Name:    "with any port",
		Options: []Option{WithAnyPort()},
		Result: BuildGuardian(
			[]string{"tcp4", "tcp6"}, nil,
			nil, nil,
			IPv4DeniedPrefixes, IPv6DeniedPrefixes,
		),
	}, {
		Name:    "with network udp6",
		Options: []Option{WithNetworks("udp6")},
		Result: BuildGuardian(
			[]string{"udp6"}, []uint16{80, 443},
			nil, nil,
			IPv4DeniedPrefixes, IPv6DeniedPrefixes,
		),
	}, {
		Name:    "with multiple network calls",
		Options: []Option{WithNetworks("tcp6"), WithNetworks("udp6")},
		Result: BuildGuardian(
			[]string{"udp6"}, []uint16{80, 443},
			nil, nil,
			IPv4DeniedPrefixes, IPv6DeniedPrefixes,
		),
	}, {
		Name:    "with network without argument",
		Options: []Option{WithNetworks()},
		Result: BuildGuardian(
			nil, []uint16{80, 443},
			nil, nil,
			IPv4DeniedPrefixes, IPv6DeniedPrefixes,
		),
	}, {
		Name:    "with any network",
		Options: []Option{WithAnyNetwork()},
		Result: BuildGuardian(
			nil, []uint16{80, 443},
			nil, nil,
			IPv4DeniedPrefixes, IPv6DeniedPrefixes,
		),
	}, {
		Name:    "with allowed v4 prefix",
		Options: []Option{WithAllowedV4Prefixes(netip.MustParsePrefix("8.8.8.0/24"))},
		Result: BuildGuardian(
			[]string{"tcp4", "tcp6"}, []uint16{80, 443},
			[]netip.Prefix{netip.MustParsePrefix("8.8.8.0/24")}, nil,
			IPv4DeniedPrefixes, IPv6DeniedPrefixes,
		),
	}, {
		Name: "with multiple allowed v4 prefix calls",
		Options: []Option{
			WithAllowedV4Prefixes(netip.MustParsePrefix("8.8.8.0/23")),
			WithAllowedV4Prefixes(netip.MustParsePrefix("8.8.8.0/24")),
		},
		Result: BuildGuardian(
			[]string{"tcp4", "tcp6"}, []uint16{80, 443},
			[]netip.Prefix{netip.MustParsePrefix("8.8.8.0/24")}, nil,
			IPv4DeniedPrefixes, IPv6DeniedPrefixes,
		),
	}, {
		Name:    "with allowed v6 prefix",
		Options: []Option{WithAllowedV6Prefixes(netip.MustParsePrefix("2002::/8"))},
		Result: BuildGuardian(
			[]string{"tcp4", "tcp6"}, []uint16{80, 443},
			nil, []netip.Prefix{netip.MustParsePrefix("2002::/8")},
			IPv4DeniedPrefixes, IPv6DeniedPrefixes,
		),
	}, {
		Name:    "with allowed NAT64 prefix",
		Options: []Option{WithAllowedV6Prefixes(IPv6NAT64Prefix)},
		Result: BuildGuardian(
			[]string{"tcp4", "tcp6"}, []uint16{80, 443},
			nil, []netip.Prefix{IPv6NAT64Prefix},
			IPv4DeniedPrefixes, IPv6DeniedPrefixes,
		),
	}, {
		Name: "with multiple allowed v6 prefix calls",
		Options: []Option{
			WithAllowedV6Prefixes(netip.MustParsePrefix("2002::/23")),
			WithAllowedV6Prefixes(netip.MustParsePrefix("2002::/8")),
		},
		Result: BuildGuardian(
			[]string{"tcp4", "tcp6"}, []uint16{80, 443},
			nil, []netip.Prefix{netip.MustParsePrefix("2002::/8")},
			IPv4DeniedPrefixes, IPv6DeniedPrefixes,
		),
	}, {
		Name:    "with denied v4 prefix",
		Options: []Option{WithDeniedV4Prefixes(netip.MustParsePrefix("8.8.8.0/24"))},
		Result: BuildGuardian(
			[]string{"tcp4", "tcp6"}, []uint16{80, 443},
			nil, nil,
			append([]netip.Prefix{netip.MustParsePrefix("8.8.8.0/24")}, IPv4DeniedPrefixes...),
			IPv6DeniedPrefixes,
		),
	}, {
		Name: "with multiple denied v4 prefix calls",
		Options: []Option{
			WithDeniedV4Prefixes(netip.MustParsePrefix("8.8.8.0/23")),
			WithDeniedV4Prefixes(netip.MustParsePrefix("8.8.8.0/24")),
		},
		Result: BuildGuardian(
			[]string{"tcp4", "tcp6"}, []uint16{80, 443},
			nil, nil,
			append([]netip.Prefix{netip.MustParsePrefix("8.8.8.0/24")}, IPv4DeniedPrefixes...),
			IPv6DeniedPrefixes,
		),
	}, {
		Name:    "with denied v6 prefix",
		Options: []Option{WithDeniedV6Prefixes(netip.MustParsePrefix("2002::/8"))},
		Result: BuildGuardian(
			[]string{"tcp4", "tcp6"}, []uint16{80, 443},
			nil, nil,
			IPv4DeniedPrefixes,
			append([]netip.Prefix{netip.MustParsePrefix("2002::/8")}, IPv6DeniedPrefixes...),
		),
	}, {
		Name: "with multiple denied v6 prefix calls",
		Options: []Option{
			WithDeniedV6Prefixes(netip.MustParsePrefix("2002::/23")),
			WithDeniedV6Prefixes(netip.MustParsePrefix("2002::/8")),
		},
		Result: BuildGuardian(
			[]string{"tcp4", "tcp6"}, []uint16{80, 443},
			nil, nil,
			IPv4DeniedPrefixes,
			append([]netip.Prefix{netip.MustParsePrefix("2002::/8")}, IPv6DeniedPrefixes...),
		),
	}}

	comparePrefix := func(t *testing.T) func(prefixA, prefixB netip.Prefix) bool {
		t.Helper()

		return func(prefixA, prefixB netip.Prefix) bool {
			switch {
			case !prefixA.IsValid() && !prefixB.IsValid(),
				prefixA.Bits() != prefixB.Bits(),
				prefixA.Addr() != prefixB.Addr():
				return false
			default:
				return true
			}
		}
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()

			if diff := cmp.Diff(
				tc.Result,
				New(tc.Options...),
				cmp.AllowUnexported(Guardian{}),
				cmp.Comparer(comparePrefix(t)),
			); diff != "" {
				t.Fatalf("Mismatch between New() and expected Guardian configuration:\n%s", diff)
			}
		})
	}
}

//nolint:gocognit // it's quite easy to understand
func TestDefaultGuardian(t *testing.T) {
	t.Parallel()

	tests := []struct {
		Addr    string
		Network string
		Err     error
	}{
		{Addr: "8.8.8.8:80", Network: "tcp4"},
		{Addr: "8.8.8.8:443", Network: "tcp4"},
		{Addr: "[2001:4860:4860::8888]:80", Network: "tcp6"},
		{Addr: "[2001:4860:4860::8888]:443", Network: "tcp6"},
		{Addr: "127.0.0.1:53", Network: "tcp4", Err: ErrProhibitedPort},
		{Addr: "127.0.0.1:80", Network: "tcp4", Err: ErrProhibitedIP},
		{Addr: "[::1]:53", Network: "tcp6", Err: ErrProhibitedPort},
		{Addr: "[::1]:80", Network: "tcp6", Err: ErrProhibitedIP},
		{Addr: "invalid network", Network: "udp6", Err: ErrProhibitedNetwork},
		{Addr: "invalid address", Network: "tcp4", Err: ErrInvalidHostPort},
		{Addr: "[::ffff:129.144.52.38]:80", Network: "tcp6", Err: ErrProhibitedIP},
		{Addr: "[64:ff9b::7f00:1]:80", Network: "tcp6", Err: ErrProhibitedIP},
	}

	filter := New()

	for _, tc := range tests {
		t.Run(fmt.Sprintf("%s_%s", tc.Network, tc.Addr), func(t *testing.T) {
			t.Parallel()

			err := filter.Safe(tc.Network, tc.Addr, nil)
			if tc.Err == nil && err != nil {
				t.Fatalf("Expected %s://%s to be permitted, got: %v", tc.Network, tc.Addr, err)
			}

			if tc.Err != nil && err == nil {
				t.Fatalf("Expected %s://%s to be denied", tc.Network, tc.Addr)
			}

			if tc.Err != nil && err != nil {
				if !errors.Is(err, tc.Err) {
					t.Fatalf("Expected error: %v, got: %v", tc.Err, err)
				}
			}
		})
	}
}

//nolint:gocognit // it's quite easy to understand
func TestCustomGuardian(t *testing.T) {
	t.Parallel()

	tests := []struct {
		Name     string
		Guardian *Guardian
		Addr     string
		Network  string
		Err      error
	}{{
		Name:     "custom port",
		Guardian: New(WithPorts(8080)),
		Addr:     "8.8.8.8:8080",
		Network:  "tcp4",
	}, {
		Name:     "any port",
		Guardian: New(WithAnyPort()),
		Addr:     "8.8.8.8:22",
		Network:  "tcp4",
	}, {
		Name:     "custom network",
		Guardian: New(WithNetworks("udp4")),
		Addr:     "8.8.8.8:80",
		Network:  "udp4",
	}, {
		Name:     "any network",
		Guardian: New(WithAnyNetwork()),
		Addr:     "8.8.8.8:80",
		Network:  "ipsec",
	}, {
		Name:     "allow prefix from IP4SpecialPurpose",
		Guardian: New(WithAllowedV4Prefixes(netip.MustParsePrefix("127.0.0.0/8"))),
		Addr:     "127.0.1.1:80",
		Network:  "tcp4",
	}, {
		Name:     "allow prefix from IP6SpecialPurpose",
		Guardian: New(WithAllowedV6Prefixes(netip.MustParsePrefix("2001::/23"))),
		Addr:     "[2001::1]:80",
		Network:  "tcp6",
	}, {
		Name:     "allow IPv6 NAT64 prefix",
		Guardian: New(WithAllowedV6Prefixes(IPv6NAT64Prefix)),
		Addr:     "[64:ff9b::7f00:1]:80",
		Network:  "tcp6",
	}, {
		Name:     "deny IPv4 prefix",
		Guardian: New(WithDeniedV4Prefixes(netip.MustParsePrefix("8.8.8.0/24"))),
		Addr:     "8.8.8.8:443",
		Network:  "tcp4",
		Err:      ErrProhibitedIP,
	}, {
		Name:     "deny IPv6 prefix",
		Guardian: New(WithDeniedV6Prefixes(netip.MustParsePrefix("2001:4800::/24"))),
		Addr:     "[2001:4860:4860::8888]:443",
		Network:  "tcp6",
		Err:      ErrProhibitedIP,
	}}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()

			err := tc.Guardian.Safe(tc.Network, tc.Addr, nil)
			if tc.Err == nil && err != nil {
				t.Fatalf("Expected %s://%s to be permitted, got: %v", tc.Network, tc.Addr, err)
			}

			if tc.Err != nil && err == nil {
				t.Fatalf("Expected %s://%s to be denied", tc.Network, tc.Addr)
			}

			if tc.Err != nil && err != nil {
				if !errors.Is(err, tc.Err) {
					t.Fatalf("Expected error: %v, got: %v", tc.Err, err)
				}
			}
		})
	}
}

func TestIPv6FastPath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		Addr string
	}{
		// 0000::/8 start and finish
		{Addr: "[::]:80"},  // unspecified address
		{Addr: "[::1]:80"}, // locahost
		{Addr: "[ff:ffff:ffff:ffff:ffff:ffff:ffff:ffff]:80"},
		// ::ffff:0:0/96 start and finish
		{Addr: "[::ffff:0:0]:80"},
		{Addr: "[::ffff:ffff:ffff]:80"},
		// 64:ff9b::/96 start and finish
		{Addr: "[64:ff9b::]:80"},
		{Addr: "[64:ff9b::0:ffff:ffff]:80"},
		// 64:ff9b:1::/48 start and finish
		{Addr: "[64:ff9b:1::]:80"},
		{Addr: "[64:ff9b:0001:ffff:ffff:ffff:ffff:ffff]:80"},
		// 100::/64 start and finish
		{Addr: "[100::]:80"},
		{Addr: "[100::ffff:ffff:ffff:ffff]:80"},
		// f000::/4 start and finish, this is the supernet containing
		// unique-local, site-local, link-scoped and multicast
		{Addr: "[f000::]:80"},
		{Addr: "[ffff:ffff:ffff:ffff:ffff:ffff:ffff:ffff]:80"},
	}

	guardian := New()

	for _, tc := range tests {
		t.Run(tc.Addr, func(t *testing.T) {
			t.Parallel()

			err := guardian.Safe("tcp6", tc.Addr, nil)
			if err == nil {
				t.Fatalf("Expected prefix: %s to be blocked", tc.Addr)
			}

			if !errors.Is(err, ErrProhibitedIP) {
				t.Log(err)
				t.Fatalf("Expected prefix: %s to be blocked with: ErrProhibitedIP", tc.Addr)
			}

			if !strings.Contains(err.Error(), "outside of the IPv6 Global Unicast range") {
				t.Log(err)
				t.Fatalf("Expected prefix: %s to be caught by Global Unicast check", tc.Addr)
			}
		})
	}
}

func prefix(s string) netip.Prefix { return netip.MustParsePrefix(s) }
