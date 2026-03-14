package ids

import (
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
)

const threadIDFormat = "users/%v/threads/%v"

type ThreadID struct {
	// unlike other ids, thread id is a string due to compatibility with
	// gateways.
	id   string
	user UserID

	_valid bool
}

func NewThreadIDFromString(s string) (ThreadID, error) {
	str := strings.Split(s, "/")
	if len(str) != 4 || str[0] != "users" || str[2] != "threads" {
		return ThreadID{}, errors.New("invalid thread id format")
	}

	uRaw, err := uuid.Parse(str[1])
	if err != nil {
		return ThreadID{}, err
	}

	u, err := NewUserID(uRaw)
	if err != nil {
		return ThreadID{}, err
	}

	return NewThreadID(u, str[3])
}

func RandomThreadID(user UserID) (ThreadID, error) {
	return NewThreadID(user, uuid.New().String())
}

func NewThreadID(user UserID, id string) (ThreadID, error) {
	u := ThreadID{
		id:   id,
		user: user,
	}
	if err := u.validate(); err != nil {
		return ThreadID{}, err
	}

	u._valid = true

	return u, nil
}

func (u ThreadID) Valid() bool { return u._valid || u.validate() == nil }
func (u ThreadID) validate() error {
	switch {
	case u.id == "":
		return errors.New("thread id cannot be empty")
	case !u.user.Valid():
		return errors.New("user id is invalid")
	default:
		return nil
	}
}

func (u ThreadID) ID() string   { return u.id }
func (u ThreadID) User() UserID { return u.user }

func (u ThreadID) String() string {
	return fmt.Sprintf(threadIDFormat, u.user.id.String(), u.id)
}
