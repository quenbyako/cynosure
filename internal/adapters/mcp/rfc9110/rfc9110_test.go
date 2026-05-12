package rfc9110_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/quenbyako/cynosure/internal/adapters/mcp/rfc9110"
)

const (
	realmKey   = "realm"
	realmValue = "example"

	schemeBearer = "bearer"
	schemeBasic  = "basic"
	paramError   = "error"
	testInvalid  = "invalid"
)

func TestParseWWWAuthenticate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		header string
		want   []rfc9110.AuthChallenge
		valid  bool
	}{{
		name:   "empty header",
		header: "",
		want:   nil,
		valid:  false,
	}, {
		name:   "link instead of format",
		header: `https://example.com`,
		want:   nil,
		valid:  false,
	}, {
		name:   "single challenge with params",
		header: `Bearer realm="example"`,
		want: []rfc9110.AuthChallenge{{
			Params: map[string]string{realmKey: realmValue},
			Scheme: schemeBearer,
			Data:   "",
		}},
		valid: true,
	}, {
		name:   "multiple challenges",
		header: `Newauth realm="apps", type=1, title="Login to \"apps\"", Basic realm="simple"`,
		want: []rfc9110.AuthChallenge{{
			Params: map[string]string{
				realmKey: "apps",
				"type":   "1",
				"title":  `Login to "apps"`,
			},
			Scheme: "newauth",
			Data:   "",
		}, {
			Params: map[string]string{realmKey: "simple"},
			Scheme: schemeBasic,
			Data:   "",
		}},
		valid: true,
	}, {
		name:   "token68 challenge",
		header: `Basic ZXhhbXBsZXRva2Vu`,
		want: []rfc9110.AuthChallenge{{
			Params: map[string]string{},
			Scheme: schemeBasic,
			Data:   "ZXhhbXBsZXRva2Vu",
		}},
		valid: true,
	}, {
		name:   "extra spaces",
		header: `  Bearer   realm  =  "example"  ,  error = "invalid"  `,
		want: []rfc9110.AuthChallenge{{
			Params: map[string]string{
				realmKey:   realmValue,
				paramError: testInvalid,
			},
			Scheme: schemeBearer,
			Data:   "",
		}},
		valid: true,
	}, {
		name:   "unquoted params",
		header: `Bearer realm=example, error=invalid_token`,
		want: []rfc9110.AuthChallenge{{
			Params: map[string]string{
				realmKey:   realmValue,
				paramError: "invalid_token",
			},
			Scheme: schemeBearer,
			Data:   "",
		}},
		valid: true,
	}, {
		name:   "multiple empty elements",
		header: `Bearer realm="a", , , , Basic`,
		want: []rfc9110.AuthChallenge{{
			Params: map[string]string{realmKey: "a"},
			Scheme: schemeBearer,
			Data:   "",
		}, {
			Params: map[string]string{},
			Scheme: schemeBasic,
			Data:   "",
		}},
		valid: true,
	}, {
		name:   "BWS around equals",
		header: `Bearer  realm = "a" , error = invalid `,
		want: []rfc9110.AuthChallenge{{
			Params: map[string]string{
				realmKey:   "a",
				paramError: testInvalid,
			},
			Scheme: schemeBearer,
			Data:   "",
		}},
		valid: true,
	}, {
		name:   "case normalization",
		header: `BEARER REALM="example", Error=invalid`,
		want: []rfc9110.AuthChallenge{{
			Params: map[string]string{
				realmKey:   realmValue,
				paramError: testInvalid,
			},
			Scheme: schemeBearer,
			Data:   "",
		}},
		valid: true,
	}, {
		name:   "case normalization, QUOTED",
		header: `BEARER REALM="example", Error="INVALID"`,
		want: []rfc9110.AuthChallenge{{
			Params: map[string]string{
				realmKey:   realmValue,
				paramError: "INVALID",
			},
			Scheme: schemeBearer,
			Data:   "",
		}},
		valid: true,
	}, {
		name:   "multiple same scheme",
		header: `Basic realm="a", Basic realm="b"`,
		want: []rfc9110.AuthChallenge{{
			Params: map[string]string{realmKey: "a"},
			Scheme: schemeBasic,
			Data:   "",
		}, {
			Params: map[string]string{realmKey: "b"},
			Scheme: schemeBasic,
			Data:   "",
		}},
		valid: true,
	}}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, ok := rfc9110.ParseWWWAuthenticate(t.Context(), tt.header)
			require.Equal(t, tt.valid, ok)
			require.Equal(t, tt.want, got)
		})
	}
}
