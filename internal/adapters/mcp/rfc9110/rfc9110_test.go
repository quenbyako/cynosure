package rfc9110_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/quenbyako/cynosure/internal/adapters/mcp/rfc9110"
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
			Params: map[string]string{"realm": "example"},
			Scheme: "bearer",
			Data:   "",
		}},
		valid: true,
	}, {
		name:   "multiple challenges",
		header: `Newauth realm="apps", type=1, title="Login to \"apps\"", Basic realm="simple"`,
		want: []rfc9110.AuthChallenge{{
			Params: map[string]string{
				"realm": "apps",
				"type":  "1",
				"title": `Login to "apps"`,
			},
			Scheme: "newauth",
			Data:   "",
		}, {
			Params: map[string]string{"realm": "simple"},
			Scheme: "basic",
			Data:   "",
		}},
		valid: true,
	}, {
		name:   "token68 challenge",
		header: `Basic ZXhhbXBsZXRva2Vu`,
		want: []rfc9110.AuthChallenge{{
			Params: map[string]string{},
			Scheme: "basic",
			Data:   "ZXhhbXBsZXRva2Vu",
		}},
		valid: true,
	}, {
		name:   "extra spaces",
		header: `  Bearer   realm  =  "example"  ,  error = "invalid"  `,
		want: []rfc9110.AuthChallenge{{
			Params: map[string]string{
				"realm": "example",
				"error": "invalid",
			},
			Scheme: "bearer",
			Data:   "",
		}},
		valid: true,
	}, {
		name:   "unquoted params",
		header: `Bearer realm=example, error=invalid_token`,
		want: []rfc9110.AuthChallenge{{
			Params: map[string]string{
				"realm": "example",
				"error": "invalid_token",
			},
			Scheme: "bearer",
			Data:   "",
		}},
		valid: true,
	}, {
		name:   "multiple empty elements",
		header: `Bearer realm="a", , , , Basic`,
		want: []rfc9110.AuthChallenge{{
			Params: map[string]string{"realm": "a"},
			Scheme: "bearer",
			Data:   "",
		}, {
			Params: map[string]string{},
			Scheme: "basic",
			Data:   "",
		}},
		valid: true,
	}, {
		name:   "BWS around equals",
		header: `Bearer  realm = "a" , error = invalid `,
		want: []rfc9110.AuthChallenge{{
			Params: map[string]string{
				"realm": "a",
				"error": "invalid",
			},
			Scheme: "bearer",
			Data:   "",
		}},
		valid: true,
	}, {
		name:   "case normalization",
		header: `BEARER REALM="example", Error=invalid`,
		want: []rfc9110.AuthChallenge{{
			Params: map[string]string{
				"realm": "example",
				"error": "invalid",
			},
			Scheme: "bearer",
			Data:   "",
		}},
		valid: true,
	}, {
		name:   "case normalization, QUOTED",
		header: `BEARER REALM="example", Error="INVALID"`,
		want: []rfc9110.AuthChallenge{{
			Params: map[string]string{
				"realm": "example",
				"error": "INVALID",
			},
			Scheme: "bearer",
			Data:   "",
		}},
		valid: true,
	}, {
		name:   "multiple same scheme",
		header: `Basic realm="a", Basic realm="b"`,
		want: []rfc9110.AuthChallenge{{
			Params: map[string]string{"realm": "a"},
			Scheme: "basic",
			Data:   "",
		}, {
			Params: map[string]string{"realm": "b"},
			Scheme: "basic",
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
