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

func StateFromToken(token string, aesKey [16]byte) (State, error) {
	tokenRaw, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		return State{}, fmt.Errorf("invalid base64: %w", err)
	}

	payload, err := decryptClaims(tokenRaw, aesKey)
	if err != nil {
		return State{}, err
	}

	state, err := stateFromClaims(&payload)
	if err != nil {
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

func (s *State) State(kid string, aesKey [16]byte) (string, error) {
	data, err := s.encryptState(kid, aesKey)
	if err != nil {
		return "", err
	}

	return base64.RawURLEncoding.EncodeToString(data), nil
}

func decryptClaims(tokenRaw []byte, aesKey [16]byte) (claims, error) {
	kk, err := aesgcm.KeyFrom(iana.AlgorithmA128GCM, aesKey[:])
	if err != nil {
		return claims{}, ErrInternalValidation("invalid key")
	}

	encryptor, err := kk.Encryptor()
	if err != nil {
		return claims{}, fmt.Errorf("creating decryption stack: %w", err)
	}

	res, err := cose.DecryptEncrypt0Message[claims](encryptor, tokenRaw, nil)
	if err != nil {
		return claims{}, fmt.Errorf("decrypting token: %w", err)
	}

	return res.Payload, nil
}

func stateFromClaims(payloadClaims *claims) (State, error) {
	user, err := ids.NewUserID(payloadClaims.UserID)
	if err != nil {
		return State{}, fmt.Errorf("parsing user id: %w", err)
	}

	server, err := ids.NewServerID(payloadClaims.ServerID)
	if err != nil {
		return State{}, fmt.Errorf("parsing server id: %w", err)
	}

	accountID, err := ids.NewAccountID(user, server, payloadClaims.AccountID)
	if err != nil {
		return State{}, fmt.Errorf("parsing account id: %w", err)
	}

	exp := payloadClaims.Expiration
	if exp > math.MaxInt64 {
		exp = math.MaxInt64
	}

	state := State{
		account: accountID, name: payloadClaims.AccountName, desc: payloadClaims.AccountDesc,
		challenge: payloadClaims.Challenge, expireAt: time.Unix(int64(exp), 0).UTC(),
		_valid: true,
	}
	if err := state.validate(); err != nil {
		return State{}, err
	}

	return state, nil
}

func (s *State) encryptState(kid string, aesKey [16]byte) ([]byte, error) {
	coseKey, err := aesgcm.KeyFrom(iana.AlgorithmA128GCM, aesKey[:])
	if err != nil {
		return nil, ErrInternalValidation("invalid state: key returned %q", err.Error())
	}

	protected := s.makeProtectedHeaders(kid)
	cl := s.makeClaims()

	msg := cose.Encrypt0Message[claims]{
		Protected:   protected,
		Unprotected: cose.Headers{},
		Payload:     cl,
	}

	encryptor, err := coseKey.Encryptor()
	if err != nil {
		return nil, fmt.Errorf("creating encryptor: %w", err)
	}

	data, err := msg.EncryptAndEncode(encryptor, nil)
	if err != nil {
		return nil, fmt.Errorf("encrypting state: %w", err)
	}

	return data, nil
}

func (s *State) makeProtectedHeaders(kid string) cose.Headers {
	protected := cose.Headers{}
	if kid != "" {
		protected[iana.HeaderParameterKid] = key.ByteStr(kid)
	}

	return protected
}

func (s *State) makeClaims() claims {
	return claims{
		Claims: cwt.Claims{
			Expiration: uint64(max(0, s.expireAt.Unix())),
			Issuer:     "",
			Subject:    "",
			Audience:   "",
			NotBefore:  0,
			IssuedAt:   0,
			CWTID:      nil,
		},
		AccountID:   s.account.ID(),
		AccountName: s.name,
		AccountDesc: s.desc,
		UserID:      s.account.User().ID(),
		ServerID:    s.account.Server().ID(),
		Challenge:   s.challenge,
	}
}
