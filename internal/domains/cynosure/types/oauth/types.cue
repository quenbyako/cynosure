package oauth

import "github.com/quenbyako/core/cue:schema"

State: schema.#ValueObject & {
	constructor: true
	fields: {
		value: {
			type: "string"
			desc: "Randomly generated string to mitigate CSRF attacks."
		}
		account: {
			type: "github.com/quenbyako/cynosure/internal/domains/cynosure/types/ids#AccountID"
			desc: "The account ID this state is associated with."
		}
		name: {
			type: "string"
			desc: "The name of the OAuth flow this state is associated with."
		}
		desc: {
			type: "string"
			desc: "A human-readable description of the OAuth state."
		}
		challenge: {
			type: "[]byte"
			desc: "PKCE challenge for enhanced security."
		}
		expireAt: {
			type: "time.Time"
			desc: "Expiration time of the OAuth state."
		}
	}
}
