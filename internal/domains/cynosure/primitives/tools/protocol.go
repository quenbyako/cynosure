// Package tools provides primitives for tool definitions and discovery.
package tools

import (
	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives"
)

// Protocol defines the communication protocol used by a tool.
//
//go:generate go tool stringer -type=Protocol -linecomment -output=protocol_string.gen.go
//nolint:recvcheck // String() is generated with value receiver by stringer
type Protocol uint8

const (
	ProtocolUnknown Protocol = iota // unknown
	ProtocolHTTP                    // http
	ProtocolSSE                     // sse
)

// ParseProtocol parses a string into a Protocol.
func ParseProtocol(str string) (res Protocol, err error) {
	err = res.UnmarshalText([]byte(str))
	return res, err
}

// UnmarshalText implements encoding.TextUnmarshaler.
func (s *Protocol) UnmarshalText(buf []byte) error {
	if s == nil {
		return primitives.ErrNilObject
	}

	for i := range len(_Protocol_index) - 1 {
		if string(buf) == _Protocol_name[_Protocol_index[i]:_Protocol_index[i+1]] {
			*s = Protocol(i)
			return nil
		}
	}

	return primitives.ErrInvalidEnum(string(buf))
}

// Valid checks if the protocol is valid.
func (s Protocol) Valid() bool {
	return s <= Protocol(len(_Protocol_index)-1)
}
