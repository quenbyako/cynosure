package rfc9110_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	. "github.com/quenbyako/cynosure/internal/adapters/mcp/rfc9110"
)

func TestLexWWWAuthenticate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  []Token
	}{{
		name:  "single challenge basic",
		input: "Bearer",
		want: []Token{
			NewToken("auth-scheme", "Bearer"),
		},
	}, {
		name:  "single challenge with param",
		input: `Bearer realm="example"`,
		want: []Token{
			NewToken("auth-scheme", "Bearer"),
			NewToken("key", "realm"),
			NewToken("value", "example"),
		},
	}, {
		name:  "token68",
		input: "Basic ZXhhbXBsZXRva2Vu==",
		want: []Token{
			NewToken("auth-scheme", "Basic"),
			NewToken("token68", "ZXhhbXBsZXRva2Vu=="),
		},
	}, {
		name:  "multiple params",
		input: `Newauth realm="apps", type=1, title="Login to \"apps\""`,
		want: []Token{
			NewToken("auth-scheme", "Newauth"),
			NewToken("key", "realm"),
			NewToken("value", "apps"),
			NewToken("key", "type"),
			NewToken("value", "1"),
			NewToken("key", "title"),
			NewToken("value", `Login to "apps"`),
		},
	}, {
		name:  "multiple challenges",
		input: `Bearer realm="example", Basic`,
		want: []Token{
			NewToken("auth-scheme", "Bearer"),
			NewToken("key", "realm"),
			NewToken("value", "example"),
			NewToken("auth-scheme", "Basic"),
		},
	}, {
		name:  "error unclosed quote",
		input: `Bearer realm="example`,
		want: []Token{
			NewToken("auth-scheme", "Bearer"),
			NewToken("key", "realm"),
			NewToken("error", "unclosed quote"),
		},
	}, {
		name:  "token68 with padding after equals",
		input: `Bearer realm=,`,
		want: []Token{
			NewToken("auth-scheme", "Bearer"),
			NewToken("token68", "realm="),
		},
	}, {
		name:  "token with plus",
		input: `Bearer error=invalid+token`,
		want: []Token{
			NewToken("auth-scheme", "Bearer"),
			NewToken("key", "error"),
			NewToken("value", "invalid+token"),
		},
	}, {
		name:  "leading comma",
		input: `, Bearer`,
		want: []Token{
			NewToken("auth-scheme", "Bearer"),
		},
	}, {
		name:  "error missing equals",
		input: `Bearer realm "example"`,
		want: []Token{
			NewToken("auth-scheme", "Bearer"),
			NewToken("token68", "realm"),
			NewToken("error", "unexpected character '\"'"),
		},
	}, {
		name:  "quoted with escape",
		input: `Newauth title="Login to \"Secure Zone\""`,
		want: []Token{
			NewToken("auth-scheme", "Newauth"),
			NewToken("key", "title"),
			NewToken("value", `Login to "Secure Zone"`),
		},
	}, {
		name:  "token68 with plus and slash",
		input: `Basic YWxhZGRpbjpvcGVuIHNlc2FtZQ/+`,
		want: []Token{
			NewToken("auth-scheme", "Basic"),
			NewToken("token68", "YWxhZGRpbjpvcGVuIHNlc2FtZQ/+"),
		},
	}, {
		name:  "multiple challenges complex",
		input: `Bearer realm="a", Basic ZXhhbXBsZQ==, Newauth tag=123`,
		want: []Token{
			NewToken("auth-scheme", "Bearer"),
			NewToken("key", "realm"),
			NewToken("value", "a"),
			NewToken("auth-scheme", "Basic"),
			NewToken("token68", "ZXhhbXBsZQ=="),
			NewToken("auth-scheme", "Newauth"),
			NewToken("key", "tag"),
			NewToken("value", "123"),
		},
	}, {
		name:  "extra whitespace everywhere",
		input: `  Bearer   realm  =  "example"  ,  Basic  `,
		want: []Token{
			NewToken("auth-scheme", "Bearer"),
			NewToken("key", "realm"),
			NewToken("value", "example"),
			NewToken("auth-scheme", "Basic"),
		},
	}, {
		name:  "unquoted value with tchars",
		input: `Newauth key=val!#$.`,
		want: []Token{
			NewToken("auth-scheme", "Newauth"),
			NewToken("key", "key"),
			NewToken("value", "val!#$."),
		},
	}, {
		name:  "trailing comma and spaces",
		input: `Bearer realm="a",   `,
		want: []Token{
			NewToken("auth-scheme", "Bearer"),
			NewToken("key", "realm"),
			NewToken("value", "a"),
		},
	}, {
		name:  "multiple commas",
		input: `Bearer realm="a",,,, Basic`,
		want: []Token{
			NewToken("auth-scheme", "Bearer"),
			NewToken("key", "realm"),
			NewToken("value", "a"),
			NewToken("auth-scheme", "Basic"),
		},
	}, {
		name:  "error missing param key",
		input: `Bearer realm="a", =val`,
		want: []Token{
			NewToken("auth-scheme", "Bearer"),
			NewToken("key", "realm"),
			NewToken("value", "a"),
			NewToken("error", "expected param key"),
		},
	}, {
		name:  "error unexpected character after challenge",
		input: `Bearer @`,
		want: []Token{
			NewToken("auth-scheme", "Bearer"),
			NewToken("token68", ""),
			NewToken("error", "unexpected character '@'"),
		},
	}, {
		name:  "token68 single padding at EOF",
		input: `Basic ZXhhbXBsZQ=`,
		want: []Token{
			NewToken("auth-scheme", "Basic"),
			NewToken("token68", "ZXhhbXBsZQ="),
		},
	}, {
		name:  "error missing space after scheme",
		input: `BasicZXhhbXBsZQ==`,
		want: []Token{
			NewToken("auth-scheme", "BasicZXhhbXBsZQ"),
			NewToken("error", "expected space after auth-scheme"),
		},
	}, {
		name:  "case insensitivity",
		input: `BEARER REALM="example"`,
		want: []Token{
			NewToken("auth-scheme", "BEARER"),
			NewToken("key", "REALM"),
			NewToken("value", "example"),
		},
	}, {
		name:  "token68 single padding with slash",
		input: `Basic ZXhhbXBsL/Q=`,
		want: []Token{
			NewToken("auth-scheme", "Basic"),
			NewToken("token68", "ZXhhbXBsL/Q="),
		},
	}, {
		name:  "error quoted backslash at EOF",
		input: `Bearer realm="abc\`,
		want: []Token{
			NewToken("auth-scheme", "Bearer"),
			NewToken("key", "realm"),
			NewToken("error", "unexpected EOF"),
		},
	}, {
		name:  "scheme with tchars",
		input: `Custom-Scheme!# realm="x"`,
		want: []Token{
			NewToken("auth-scheme", "Custom-Scheme!#"),
			NewToken("key", "realm"),
			NewToken("value", "x"),
		},
	}, {
		name:  "multiple challenges with token68 and params",
		input: `Basic ZXhhbXBsZQ==, Bearer realm="token"`,
		want: []Token{
			NewToken("auth-scheme", "Basic"),
			NewToken("token68", "ZXhhbXBsZQ=="),
			NewToken("auth-scheme", "Bearer"),
			NewToken("key", "realm"),
			NewToken("value", "token"),
		},
	}}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			items := Lex(tt.input)

			var got []Token

			for item := range items {
				got = append(got, item)
			}

			require.Equal(t, tt.want, got)
		})
	}
}
