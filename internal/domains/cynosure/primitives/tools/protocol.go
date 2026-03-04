package tools

import "github.com/quenbyako/cynosure/internal/domains/cynosure/primitives"

// ToolChoice controls how the model calls tools (if any). By default, tools are
// forbidden.
//
//go:generate go tool stringer -type=Protocol -linecomment -output=protocol_string.gen.go
type Protocol uint8

const (
	ProtocolUnknown Protocol = iota // unknown
	ProtocolHTTP                    // http
	ProtocolSSE                     // sse
)

func ParseProtocol(str string) (res Protocol, err error) {
	return res, res.UnmarshalText([]byte(str))
}

func (s *Protocol) UnmarshalText(buf []byte) error {
	if s == nil {
		return primitives.ErrNilObject
	}

	for i := range len(_Protocol_index) - 1 {
		if string(buf) == _Protocol_name[_Protocol_index[i]:_Protocol_index[i+1]] {
			*s = Protocol(i) //nolint:gosec // codegen won't allow to get overflow
			return nil
		}
	}

	return primitives.ErrInvalidEnum(string(buf))
}

// IsValid checks if the tool choice is valid.
func (w Protocol) Valid() bool { return w <= Protocol(len(_Protocol_index)-1) }
