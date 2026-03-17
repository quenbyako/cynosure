package rfc9110 //nolint:testpackage // helper package for tests

type Token = token

func NewToken(typ, value string) Token {
	return Token{typ: typ, value: value}
}

func Lex(input string) chan token {
	return lex(input, lexChallenge)
}
