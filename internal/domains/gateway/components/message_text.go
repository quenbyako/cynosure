package components

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
	return nil
}

func (m MessageText) Text() string { return m.text }
