package ids

type UserID struct {
	providerID string
	userID     string

	valid bool
}

func NewUserID(providerID, userID string) (UserID, error) {
	u := UserID{
		providerID: providerID,
		userID:     userID,
	}

	if err := u.validate(); err != nil {
		return UserID{}, err
	}
	u.valid = true

	return u, nil
}

func (u UserID) Valid() bool { return u.valid || u.validate() == nil }
func (u UserID) validate() error {
	return nil
}

func (u UserID) ProviderID() string { return u.providerID }
func (u UserID) UserID() string     { return u.userID }
