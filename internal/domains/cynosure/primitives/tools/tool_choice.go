package tools

import (
	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives"
)

// ToolChoice controls how the model calls tools (if any).
//
//go:generate go tool stringer -type=ToolChoice -linecomment -output=tool_choice_string.gen.go
//nolint:recvcheck // String() is generated with value receiver by stringer
type ToolChoice uint8

const (
	_                   ToolChoice = iota
	ToolChoiceAllowed              // allowed
	ToolChoiceForbidden            // forbidden
	ToolChoiceForced               // forced
)

// ParseToolChoice parses a string into a ToolChoice.
func ParseToolChoice(str string) (res ToolChoice, err error) {
	err = res.UnmarshalText([]byte(str))
	return res, err
}

// UnmarshalText implements encoding.TextUnmarshaler.
func (s *ToolChoice) UnmarshalText(buf []byte) error {
	if s == nil {
		return primitives.ErrNilObject
	}

	for i := range len(_ToolChoice_index) - 1 {
		if string(buf) == _ToolChoice_name[_ToolChoice_index[i]:_ToolChoice_index[i+1]] {
			*s = ToolChoice(i + 1)
			return nil
		}
	}

	return primitives.ErrInvalidEnum(string(buf))
}

// Valid checks if the tool choice is valid.
func (s ToolChoice) Valid() bool {
	return s > 0 && s < ToolChoice(len(_ToolChoice_index)-1)
}
