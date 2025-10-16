package ids

type ChannelID struct {
	providerID string
	channelID  string // either chat or channel

	valid bool
}

func NewChannelID(providerID, channelID string) (ChannelID, error) {
	m := ChannelID{
		providerID: providerID,
		channelID:  channelID,
	}

	if err := m.validate(); err != nil {
		return ChannelID{}, err
	}
	m.valid = true

	return m, nil
}

func (m ChannelID) Valid() bool { return m.valid || m.validate() == nil }
func (m ChannelID) validate() error {
	return nil
}

func (m ChannelID) ProviderID() string { return m.providerID }
func (m ChannelID) ChannelID() string  { return m.channelID }

func (m ChannelID) String() string {
	return m.providerID + "/" + m.channelID
}
