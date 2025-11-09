package components

import "errors"

type MessageText struct {
	text string

	valid bool
}

func NewMessageText(text string) (MessageText, error) {
	m := MessageText{
		text: text,
	}

	if err := m.validate(); err != nil {
		return MessageText{}, err
	}
	m.valid = true

	return m, nil
}

func (m MessageText) Valid() bool { return m.valid || m.validate() == nil }
func (m MessageText) validate() error {
	if m.text == "" {
		return errors.New("empty text")
	}
	if len(m.text) > 8000 {
		return errors.New("text exceeds maximum length")
	}

	return nil
}

func (m MessageText) Text() string { return m.text }
