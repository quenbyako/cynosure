package rfc9110

import "strings"

// AuthChallenge represents a single challenge in WWW-Authenticate header.
type AuthChallenge struct {
	Scheme string
	Data   string
	Params map[string]string
}

// ParseWWWAuthenticate parses WWW-Authenticate header according to RFC 9110.
func ParseWWWAuthenticate(header string) ([]AuthChallenge, bool) {

	header = strings.TrimSpace(header)
	if header == "" {
		return nil, false
	}

	items := lex(header, lexChallenge)
	return collectChallenges(items)
}

func collectChallenges(items chan token) ([]AuthChallenge, bool) {
	var challenges []AuthChallenge
	var cur *AuthChallenge
	var lastKey string

	for it := range items {
		switch it.typ {
		case "error":
			return nil, false
		case "auth-scheme":
			challenges = append(challenges, AuthChallenge{
				Scheme: strings.ToLower(it.value),
				Params: make(map[string]string),
			})
			cur = &challenges[len(challenges)-1]
			lastKey = ""
		case "token68":
			if cur != nil {
				cur.Data = it.value
			}
		case "key":
			lastKey = strings.ToLower(it.value)
		case "value":
			if cur != nil && lastKey != "" {
				cur.Params[lastKey] = it.value
				lastKey = ""
			}
		}
	}

	return challenges, len(challenges) > 0
}
