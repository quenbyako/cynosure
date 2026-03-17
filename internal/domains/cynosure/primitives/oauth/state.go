// Package oauth defines OAuth primitives.
package oauth

import (
	"encoding/base64"
	"fmt"
	"math"
	"time"

	"github.com/google/uuid"
	"github.com/ldclabs/cose/cose"
	"github.com/ldclabs/cose/cwt"
	"github.com/ldclabs/cose/iana"
	"github.com/ldclabs/cose/key"
	"github.com/ldclabs/cose/key/aesgcm"

	"github.com/quenbyako/cynosure/internal/domains/cynosure/primitives/ids"
)

type State struct {
	expireAt  time.Time
	name      string
	desc      string
	challenge []byte
	account   ids.AccountID
	_valid    bool
}

func NewState(
	account ids.AccountID,
	name string,
	desc string,
	challenge []byte,
	expireAt time.Time,
) (State, error) {
	state := State{
		account:   account,
		name:      name,
		desc:      desc,
		challenge: challenge,
		expireAt:  expireAt,
		_valid:    false,
	}
	if err := state.validate(); err != nil {
		return State{}, err
	}

	state._valid = true

	return state, nil
}

func StateFromToken(token string, key [16]byte) (State, error) {
	tokenRaw, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		return State{}, fmt.Errorf("invalid base64: %w", err)
	}

	kk, err := aesgcm.KeyFrom(iana.AlgorithmA128GCM, key[:])
	if err != nil {
		return State{}, ErrInternalValidation("invalid key")
	}

	encryptor, err := kk.Encryptor()
	if err != nil {
		return State{}, fmt.Errorf("creating decription stack: %w", err)
	}

	res, err := cose.DecryptEncrypt0Message[claims](encryptor, tokenRaw, nil)
	if err != nil {
		return State{}, fmt.Errorf("decrypting token: %w", err)
	}

	user, err := ids.NewUserID(res.Payload.UserID)
	if err != nil {
		return State{}, fmt.Errorf("parsing user id: %w", err)
	}

	server, err := ids.NewServerID(res.Payload.ServerID)
	if err != nil {
		return State{}, fmt.Errorf("parsing server id: %w", err)
	}

	accountID, err := ids.NewAccountID(user, server, res.Payload.AccountID)
	if err != nil {
		return State{}, fmt.Errorf("parsing account id: %w", err)
	}

	expirationUnix := res.Payload.Expiration
	if expirationUnix > math.MaxInt64 {
		expirationUnix = math.MaxInt64
	}

	state := State{
		account:   accountID,
		name:      res.Payload.AccountName,
		desc:      res.Payload.AccountDesc,
		challenge: res.Payload.Challenge,
		expireAt:  time.Unix(int64(expirationUnix), 0).UTC(),
		_valid:    false,
	}
	if err := state.validate(); err != nil {
		return State{}, err
	}

	state._valid = true

	return state, nil
}

const (
	maxDescriptionLength = 100
)

func (s *State) Valid() bool { return s._valid || s.validate() == nil }
func (s *State) validate() error {
	switch {
	case s.name == "":
		return ErrInternalValidation("name is required")
	case s.desc == "":
		return ErrInternalValidation("description is required")
	case len(s.desc) > maxDescriptionLength:
		return ErrInternalValidation("description must be 100 characters or less")
	}

	return nil
}

type claims struct {
	cwt.Claims
	AccountName string    `cbor:"-2,keyasint,omitempty" json:"name,omitempty"`
	AccountDesc string    `cbor:"-3,keyasint,omitempty" json:"desc,omitempty"`
	Challenge   []byte    `cbor:"-6,keyasint,omitempty" json:"ch,omitempty"`
	AccountID   uuid.UUID `cbor:"-1,keyasint,omitempty" json:"acc,omitempty"`
	UserID      uuid.UUID `cbor:"-4,keyasint,omitempty" json:"uid,omitempty"`
	ServerID    uuid.UUID `cbor:"-5,keyasint,omitempty" json:"srv,omitempty"`
}

func (s *State) Account() ids.AccountID { return s.account }
func (s *State) Name() string           { return s.name }
func (s *State) Description() string    { return s.desc }
func (s *State) Challenge() []byte      { return s.challenge }

func (s *State) State(kid string, k [16]byte) (string, error) {
	aesKey, err := aesgcm.KeyFrom(iana.AlgorithmA128GCM, k[:])
	if err != nil {
		return "", ErrInternalValidation("invalid state: key returned %q", err.Error())
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
				Expiration: uint64(max(0, s.expireAt.Unix())),
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
		Unprotected: nil,
	}

	encryptor, err := aesKey.Encryptor()
	if err != nil {
		return "", fmt.Errorf("creating encryptor: %w", err)
	}

	data, err := msg.EncryptAndEncode(encryptor, nil)
	if err != nil {
		return "", fmt.Errorf("encrypting state: %w", err)
	}

	return base64.RawURLEncoding.EncodeToString(data), nil
}
