package tools

import "tg-helper/internal/domains/components"

// ToolChoice controls how the model calls tools (if any). By default, tools are
// forbidden.
//
//go:generate stringer -type=ToolChoice -linecomment -output=tool_choice_string.gen.go
type ToolChoice uint8

const (
	_                   ToolChoice = iota
	ToolChoiceForbidden            // forbidden
	ToolChoiceAllowed              // allowed
	ToolChoiceForced               // forced
)

func ParseToolChoice(str string) (res ToolChoice, err error) {
	return res, res.UnmarshalText([]byte(str))
}

func (s *ToolChoice) UnmarshalText(buf []byte) error {
	if s == nil {
		return components.ErrNilObject
	}

	for i := range len(_ToolChoice_index) - 1 {
		if string(buf) == _ToolChoice_name[_ToolChoice_index[i]:_ToolChoice_index[i+1]] {
			*s = ToolChoice(i + 1) //nolint:gosec // codegen won't allow to get overflow
			return nil
		}
	}

	return components.ErrInvalidEnum(string(buf))
}

// IsValid checks if the tool choice is valid.
func (w ToolChoice) Valid() bool { return w > 0 && w < ToolChoice(len(_ToolChoice_index)-1) }
