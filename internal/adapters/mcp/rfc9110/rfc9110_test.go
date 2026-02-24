package rfc9110_test

import (
	"testing"

	. "github.com/quenbyako/cynosure/internal/adapters/mcp/rfc9110"
	"github.com/stretchr/testify/require"
)

func TestParseWWWAuthenticate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		header string
		want   []AuthChallenge
		valid  bool
	}{{
		name:   "empty header",
		header: "",
		valid:  false,
	}, {
		name:   "link instead of format",
		header: `https://example.com`,
		valid:  false,
	}, {
		name:   "single challenge with params",
		header: `Bearer realm="example"`,
		want: []AuthChallenge{{
			Scheme: "bearer",
			Params: map[string]string{"realm": "example"},
		}},
		valid: true,
	}, {
		name:   "multiple challenges",
		header: `Newauth realm="apps", type=1, title="Login to \"apps\"", Basic realm="simple"`,
		want: []AuthChallenge{{
			Scheme: "newauth",
			Params: map[string]string{
				"realm": "apps",
				"type":  "1",
				"title": `Login to "apps"`,
			},
		}, {
			Scheme: "basic",
			Params: map[string]string{"realm": "simple"},
		}},
		valid: true,
	}, {
		name:   "token68 challenge",
		header: `Basic ZXhhbXBsZXRva2Vu`,
		want: []AuthChallenge{{
			Scheme: "basic",
			Data:   "ZXhhbXBsZXRva2Vu",
			Params: map[string]string{},
		}},
		valid: true,
	}, {
		name:   "extra spaces",
		header: `  Bearer   realm  =  "example"  ,  error = "invalid"  `,
		want: []AuthChallenge{{
			Scheme: "bearer",
			Params: map[string]string{
				"realm": "example",
				"error": "invalid",
			},
		}},
		valid: true,
	}, {
		name:   "unquoted params",
		header: `Bearer realm=example, error=invalid_token`,
		want: []AuthChallenge{{
			Scheme: "bearer",
			Params: map[string]string{
				"realm": "example",
				"error": "invalid_token",
			},
		}},
		valid: true,
	}, {
		name:   "multiple empty elements",
		header: `Bearer realm="a", , , , Basic`,
		want: []AuthChallenge{{
			Scheme: "bearer",
			Params: map[string]string{"realm": "a"},
		}, {
			Scheme: "basic",
			Params: map[string]string{},
		}},
		valid: true,
	}, {
		name:   "BWS around equals",
		header: `Bearer  realm = "a" , error = invalid `,
		want: []AuthChallenge{{
			Scheme: "bearer",
			Params: map[string]string{
				"realm": "a",
				"error": "invalid",
			},
		}},
		valid: true,
	}, {
		name:   "case normalization",
		header: `BEARER REALM="example", Error=invalid`,
		want: []AuthChallenge{{
			Scheme: "bearer",
			Params: map[string]string{
				"realm": "example",
				"error": "invalid",
			},
		}},
		valid: true,
	}, {
		name:   "case normalization, QUOTED",
		header: `BEARER REALM="example", Error="INVALID"`,
		want: []AuthChallenge{{
			Scheme: "bearer",
			Params: map[string]string{
				"realm": "example",
				"error": "INVALID",
			},
		}},
		valid: true,
	}, {
		name:   "multiple same scheme",
		header: `Basic realm="a", Basic realm="b"`,
		want: []AuthChallenge{{
			Scheme: "basic",
			Params: map[string]string{"realm": "a"},
		}, {
			Scheme: "basic",
			Params: map[string]string{"realm": "b"},
		}},
		valid: true,
	}}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, ok := ParseWWWAuthenticate(tt.header)
			require.Equal(t, tt.valid, ok)
			require.Equal(t, tt.want, got)
		})
	}
}
