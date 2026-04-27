// Package rfc9110 provides a parser for the WWW-Authenticate header as defined
// in RFC 9110.
package rfc9110

import (
	"context"
	"strings"
)

// AuthChallenge represents a single challenge in WWW-Authenticate header.
type AuthChallenge struct {
	Params map[string]string
	Scheme string
	Data   string
}

// ParseWWWAuthenticate parses WWW-Authenticate header according to RFC 9110.
func ParseWWWAuthenticate(ctx context.Context, header string) ([]AuthChallenge, bool) {
	header = strings.TrimSpace(header)
	if header == "" {
		return nil, false
	}

	items := lex(ctx, header, lexChallenge)

	return collectChallenges(items)
}

func collectChallenges(items chan token) ([]AuthChallenge, bool) {
	var (
		challenges []AuthChallenge
		cur        *AuthChallenge
		lastKey    string
	)

	for item := range items {
		if item.typ == "error" {
			return nil, false
		}

		if item.typ == "auth-scheme" {
			cur = handleAuthScheme(&challenges, item.value)
			lastKey = ""

			continue
		}

		handleChallengeItem(item, cur, &lastKey)
	}

	return challenges, len(challenges) > 0
}

func handleAuthScheme(challenges *[]AuthChallenge, value string) *AuthChallenge {
	*challenges = append(*challenges, AuthChallenge{
		Params: make(map[string]string),
		Scheme: strings.ToLower(value),
		Data:   "",
	})

	return &(*challenges)[len(*challenges)-1]
}

func handleChallengeItem(item token, cur *AuthChallenge, lastKey *string) {
	if cur == nil {
		return
	}

	switch item.typ {
	case "token68":
		cur.Data = item.value
	case "key":
		*lastKey = strings.ToLower(item.value)
	case "value":
		if *lastKey != "" {
			cur.Params[*lastKey] = item.value
			*lastKey = ""
		}
	}
}
