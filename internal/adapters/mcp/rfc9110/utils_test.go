package rfc9110 //nolint:testpackage // helper package for tests

import (
	"context"
)

type Token = token

func NewToken(typ, value string) Token {
	return Token{typ: typ, value: value}
}

func Lex(ctx context.Context, input string) chan token {
	return lex(ctx, input, lexChallenge)
}
