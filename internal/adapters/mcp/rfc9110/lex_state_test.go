package rfc9110

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLexWWWAuthenticate(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []token
	}{{
		name:  "single challenge basic",
		input: "Bearer",
		want: []token{
			{typ: "auth-scheme", value: "Bearer"},
		},
	}, {
		name:  "single challenge with param",
		input: `Bearer realm="example"`,
		want: []token{
			{typ: "auth-scheme", value: "Bearer"},
			{typ: "key", value: "realm"},
			{typ: "value", value: "example"},
		},
	}, {
		name:  "token68",
		input: "Basic ZXhhbXBsZXRva2Vu==",
		want: []token{
			{typ: "auth-scheme", value: "Basic"},
			{typ: "token68", value: "ZXhhbXBsZXRva2Vu=="},
		},
	}, {
		name:  "multiple params",
		input: `Newauth realm="apps", type=1, title="Login to \"apps\""`,
		want: []token{
			{typ: "auth-scheme", value: "Newauth"},
			{typ: "key", value: "realm"},
			{typ: "value", value: "apps"},
			{typ: "key", value: "type"},
			{typ: "value", value: "1"},
			{typ: "key", value: "title"},
			{typ: "value", value: `Login to "apps"`},
		},
	}, {
		name:  "multiple challenges",
		input: `Bearer realm="example", Basic`,
		want: []token{
			{typ: "auth-scheme", value: "Bearer"},
			{typ: "key", value: "realm"},
			{typ: "value", value: "example"},
			{typ: "auth-scheme", value: "Basic"},
		},
	}, {
		name:  "error unclosed quote",
		input: `Bearer realm="example`,
		want: []token{
			{typ: "auth-scheme", value: "Bearer"},
			{typ: "key", value: "realm"},
			{typ: "error", value: "unclosed quote"},
		},
	}, {
		name:  "token68 with padding after equals",
		input: `Bearer realm=,`,
		want: []token{
			{typ: "auth-scheme", value: "Bearer"},
			{typ: "token68", value: "realm="},
		},
	}, {
		name:  "token with plus",
		input: `Bearer error=invalid+token`,
		want: []token{
			{typ: "auth-scheme", value: "Bearer"},
			{typ: "key", value: "error"},
			{typ: "value", value: "invalid+token"},
		},
	}, {
		name:  "leading comma",
		input: `, Bearer`,
		want: []token{
			{typ: "auth-scheme", value: "Bearer"},
		},
	}, {
		name:  "error missing equals",
		input: `Bearer realm "example"`,
		want: []token{
			{typ: "auth-scheme", value: "Bearer"},
			{typ: "token68", value: "realm"},
			{typ: "error", value: "unexpected character '\"'"},
		},
	}, {
		name:  "quoted with escape",
		input: `Newauth title="Login to \"Secure Zone\""`,
		want: []token{
			{typ: "auth-scheme", value: "Newauth"},
			{typ: "key", value: "title"},
			{typ: "value", value: `Login to "Secure Zone"`},
		},
	}, {
		name:  "token68 with plus and slash",
		input: `Basic YWxhZGRpbjpvcGVuIHNlc2FtZQ/+`,
		want: []token{
			{typ: "auth-scheme", value: "Basic"},
			{typ: "token68", value: "YWxhZGRpbjpvcGVuIHNlc2FtZQ/+"},
		},
	}, {
		name:  "multiple challenges complex",
		input: `Bearer realm="a", Basic ZXhhbXBsZQ==, Newauth tag=123`,
		want: []token{
			{typ: "auth-scheme", value: "Bearer"},
			{typ: "key", value: "realm"},
			{typ: "value", value: "a"},
			{typ: "auth-scheme", value: "Basic"},
			{typ: "token68", value: "ZXhhbXBsZQ=="},
			{typ: "auth-scheme", value: "Newauth"},
			{typ: "key", value: "tag"},
			{typ: "value", value: "123"},
		},
	}, {
		name:  "extra whitespace everywhere",
		input: `  Bearer   realm  =  "example"  ,  Basic  `,
		want: []token{
			{typ: "auth-scheme", value: "Bearer"},
			{typ: "key", value: "realm"},
			{typ: "value", value: "example"},
			{typ: "auth-scheme", value: "Basic"},
		},
	}, {
		name:  "unquoted value with tchars",
		input: `Newauth key=val!#$.`,
		want: []token{
			{typ: "auth-scheme", value: "Newauth"},
			{typ: "key", value: "key"},
			{typ: "value", value: "val!#$."},
		},
	}, {
		name:  "trailing comma and spaces",
		input: `Bearer realm="a",   `,
		want: []token{
			{typ: "auth-scheme", value: "Bearer"},
			{typ: "key", value: "realm"},
			{typ: "value", value: "a"},
		},
	}, {
		name:  "multiple commas",
		input: `Bearer realm="a",,,, Basic`,
		want: []token{
			{typ: "auth-scheme", value: "Bearer"},
			{typ: "key", value: "realm"},
			{typ: "value", value: "a"},
			{typ: "auth-scheme", value: "Basic"},
		},
	}, {
		name:  "error missing param key",
		input: `Bearer realm="a", =val`,
		want: []token{
			{typ: "auth-scheme", value: "Bearer"},
			{typ: "key", value: "realm"},
			{typ: "value", value: "a"},
			{typ: "error", value: "expected param key"},
		},
	}, {
		name:  "error unexpected character after challenge",
		input: `Bearer @`,
		want: []token{
			{typ: "auth-scheme", value: "Bearer"},
			{typ: "token68", value: ""},
			{typ: "error", value: "unexpected character '@'"},
		},
	}, {
		name:  "token68 single padding at EOF",
		input: `Basic ZXhhbXBsZQ=`,
		want: []token{
			{typ: "auth-scheme", value: "Basic"},
			{typ: "token68", value: "ZXhhbXBsZQ="},
		},
	}, {
		name:  "error missing space after scheme",
		input: `BasicZXhhbXBsZQ==`,
		want: []token{
			{typ: "auth-scheme", value: "BasicZXhhbXBsZQ"},
			{typ: "error", value: "expected space after auth-scheme"},
		},
	}, {
		name:  "case insensitivity",
		input: `BEARER REALM="example"`,
		want: []token{
			{typ: "auth-scheme", value: "BEARER"},
			{typ: "key", value: "REALM"},
			{typ: "value", value: "example"},
		},
	}, {
		name:  "token68 single padding with slash",
		input: `Basic ZXhhbXBsL/Q=`,
		want: []token{
			{typ: "auth-scheme", value: "Basic"},
			{typ: "token68", value: "ZXhhbXBsL/Q="},
		},
	}, {
		name:  "error quoted backslash at EOF",
		input: `Bearer realm="abc\`,
		want: []token{
			{typ: "auth-scheme", value: "Bearer"},
			{typ: "key", value: "realm"},
			{typ: "error", value: "unexpected EOF"},
		},
	}, {
		name:  "scheme with tchars",
		input: `Custom-Scheme!# realm="x"`,
		want: []token{
			{typ: "auth-scheme", value: "Custom-Scheme!#"},
			{typ: "key", value: "realm"},
			{typ: "value", value: "x"},
		},
	}, {
		name:  "multiple challenges with token68 and params",
		input: `Basic ZXhhbXBsZQ==, Bearer realm="token"`,
		want: []token{
			{typ: "auth-scheme", value: "Basic"},
			{typ: "token68", value: "ZXhhbXBsZQ=="},
			{typ: "auth-scheme", value: "Bearer"},
			{typ: "key", value: "realm"},
			{typ: "value", value: "token"},
		},
	}}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			items := lex(tt.input, lexChallenge)
			var got []token
			for item := range items {
				got = append(got, item)
			}
			require.Equal(t, tt.want, got)
		})
	}
}
