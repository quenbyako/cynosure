package ids

type MessageID struct {
	channelID ChannelID
	messageID string

	valid bool
}

func NewMessageID(channelID ChannelID, messageID string) (MessageID, error) {
	m := MessageID{
		channelID: channelID,
		messageID: messageID,
	}

	if err := m.validate(); err != nil {
		return MessageID{}, err
	}
	m.valid = true

	return m, nil
}

func (m MessageID) Valid() bool { return m.valid || m.validate() == nil }
func (m MessageID) validate() error {
	return nil
}

func (m MessageID) ChannelID() ChannelID { return m.channelID }
func (m MessageID) MessageID() string    { return m.messageID }

func (m MessageID) String() string {
	return m.channelID.String() + "/" + m.messageID
}
