package oauth

import (
	"encoding/base64"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/ldclabs/cose/cose"
	"github.com/ldclabs/cose/cwt"
	"github.com/ldclabs/cose/iana"
	"github.com/ldclabs/cose/key"
	"github.com/ldclabs/cose/key/aesgcm"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/types/ids"
)

type State struct {
	account   ids.AccountID
	name      string
	desc      string
	challenge []byte
	ExpireAt  time.Time

	valid bool
}

func NewState(
	account ids.AccountID,
	name string,
	desc string,
	challenge []byte,
	expireAt time.Time,
) (State, error) {
	s := State{
		account:   account,
		name:      name,
		desc:      desc,
		challenge: challenge,
		ExpireAt:  expireAt,
	}
	if err := s.validate(); err != nil {
		return State{}, err
	}
	s.valid = true

	return s, nil
}

func StateFromToken(token string, k [16]byte) (State, error) {
	tokenRaw, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		return State{}, fmt.Errorf("invalid base64: %w", err)
	}

	kk, err := aesgcm.KeyFrom(iana.AlgorithmA128GCM, k[:])
	if err != nil {
		panic(err)
	}

	res, err := cose.DecryptEncrypt0Message[claims](must(kk.Encryptor()), tokenRaw, nil)
	if err != nil {
		return State{}, fmt.Errorf("decrypting token: %w", err)
	}

	user := must(ids.NewUserID(res.Payload.UserID))
	server := must(ids.NewServerID(res.Payload.ServerID))

	s := State{
		account:   must(ids.NewAccountID(user, server, res.Payload.AccountID)),
		name:      res.Payload.AccountName,
		desc:      res.Payload.AccountDesc,
		challenge: res.Payload.Challenge,
		ExpireAt:  time.Unix(int64(res.Payload.Expiration), 0).UTC(),
	}
	if err := s.validate(); err != nil {
		return State{}, err
	}

	s.valid = true

	return s, nil
}

func (s *State) Valid() bool { return s.valid || s.validate() == nil }
func (s *State) validate() error {
	switch {
	case s.name == "":
		return fmt.Errorf("name is required")
	case s.desc == "":
		return fmt.Errorf("description is required")
	case len(s.desc) > 100:
		return fmt.Errorf("description must be 100 characters or less")
	}
	return nil
}

type claims struct {
	cwt.Claims

	AccountID   uuid.UUID `cbor:"-1,keyasint,omitempty" json:"acc,omitempty"`
	AccountName string    `cbor:"-2,keyasint,omitempty" json:"name,omitempty"`
	AccountDesc string    `cbor:"-3,keyasint,omitempty" json:"desc,omitempty"`
	UserID      uuid.UUID `cbor:"-4,keyasint,omitempty" json:"uid,omitempty"`
	ServerID    uuid.UUID `cbor:"-5,keyasint,omitempty" json:"srv,omitempty"`
	Challenge   []byte    `cbor:"-6,keyasint,omitempty" json:"ch,omitempty"`
}

func (s *State) Account() ids.AccountID { return s.account }
func (s *State) Name() string           { return s.name }
func (s *State) Description() string    { return s.desc }
func (s *State) Challenge() []byte      { return s.challenge }

func (s *State) State(kid string, k [16]byte) string {
	kk, err := aesgcm.KeyFrom(iana.AlgorithmA128GCM, k[:])
	if err != nil {
		panic(err)
	}

	protectedValues := cose.Headers{}
	if kid != "" {
		protectedValues[iana.HeaderParameterKid] = key.ByteStr(kid)
	}

	msg := cose.Encrypt0Message[claims]{
		Protected: protectedValues,
		Payload: claims{
			Claims: cwt.Claims{
				Issuer:     "",
				Subject:    "",
				Audience:   "",
				Expiration: uint64(s.ExpireAt.Unix()),
				NotBefore:  0,
				IssuedAt:   0,
				CWTID:      key.ByteStr(nil),
			},
			AccountID:   s.account.ID(),
			AccountName: s.name,
			AccountDesc: s.desc,
			UserID:      s.account.User().ID(),
			ServerID:    s.account.Server().ID(),
			Challenge:   s.challenge,
		},
	}

	data := must(msg.EncryptAndEncode(must(kk.Encryptor()), nil))

	return base64.RawURLEncoding.EncodeToString(data)
}

func must[T any](v T, err error) T {
	if err != nil {
		panic(err)
	}
	return v
}
