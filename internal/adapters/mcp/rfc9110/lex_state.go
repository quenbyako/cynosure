package rfc9110

import (
	"context"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/rangetable"
)

var (
	letters  = rangetable.New([]rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")...)
	digits   = rangetable.New([]rune("0123456789")...)
	alphanum = rangetable.Merge(letters, digits)
	// tchar as defined in RFC 9110 Section 5.6.2
	rfc7230TChar = rangetable.Merge(alphanum, rangetable.New([]rune("!#$%&'*+-.^_`|~+")...))
	// token68 as defined in RFC 9110 Section 11.2
	base68 = rangetable.Merge(alphanum, rangetable.New([]rune("-._~+/")...))

	tokensBoth = rangetable.Merge(rfc7230TChar, base68)

	equal = rangetable.New('=')
)

func lexChallenge(ctx context.Context, lex *lexer) stateFn {
	lex.acceptRun(unicode.Space)
	lex.ignore()

	// Skip empty list elements (leading or between challenges)
	for lex.peek() == ',' {
		lex.next()
		lex.ignore()
		lex.acceptRun(unicode.Space)
		lex.ignore()
	}

	if lex.eof() {
		return nil
	}

	if !lex.acceptRun(rfc7230TChar) {
		return lex.errorf(ctx, "invalid format, challenge must start with token")
	}

	lex.emit(ctx, "auth-scheme")

	return lexAfterScheme
}

func lexAfterScheme(ctx context.Context, lex *lexer) stateFn {
	hasSpace := lex.acceptRun(unicode.Space)
	lex.ignore()

	if lex.eof() {
		return nil
	}

	if lex.peek() == ',' {
		lex.next()
		lex.ignore()

		return lexChallenge
	}

	if !hasSpace {
		return lex.errorf(ctx, "expected space after auth-scheme")
	}

	return handleChallengeBody(ctx, lex)
}

func handleChallengeBody(ctx context.Context, lex *lexer) stateFn {
	// It's either a token68 or the start of auth-params
	lex.acceptRun(tokensBoth)

	tokPos := lex.pos

	lex.acceptRun(unicode.Space)

	if lex.peek() == '=' {
		return handleEqualAfterScheme(ctx, lex, tokPos)
	}

	lex.pos = tokPos

	lex.emit(ctx, "token68")

	return lexAfterChallengeBody
}

func handleEqualAfterScheme(ctx context.Context, lex *lexer, tokPos int) stateFn {
	lex.next()

	if lex.peek() == '=' {
		lex.next()
		lex.acceptRun(equal)
		lex.emit(ctx, "token68")

		return lexAfterChallengeBody
	}

	lex.acceptRun(unicode.Space)

	peeked := lex.peek()
	if peeked == ',' || peeked == -1 {
		lex.emit(ctx, "token68")

		return lexAfterChallengeBody
	}

	return lexKeyAfterScheme(ctx, lex, tokPos, peeked)
}

func lexKeyAfterScheme(ctx context.Context, lex *lexer, tokPos int, peeked rune) stateFn {
	tokenVal := string(lex.input[lex.start:tokPos])

	if !strings.Contains(tokenVal, "/") && (unicode.Is(rfc7230TChar, peeked) || peeked == '"') {
		lex.pos = tokPos
		lex.emit(ctx, "key")
		lex.acceptRun(unicode.Space)
		lex.next() // consume the '='
		lex.ignore()

		return lexValue
	}

	lex.emit(ctx, "token68")

	return lexAfterChallengeBody
}

func lexValue(ctx context.Context, lex *lexer) stateFn {
	lex.acceptRun(unicode.Space)
	lex.ignore()

	if lex.peek() == '"' {
		return lexQuotedValue(ctx, lex)
	}

	return lexTokenValue(ctx, lex)
}

func lexQuotedValue(ctx context.Context, lex *lexer) stateFn {
	if err := emitQuotedString(ctx, lex, "value"); err != nil {
		return lex.errorf(ctx, "%v", err)
	}

	return lexAfterChallengeBody
}

func lexTokenValue(ctx context.Context, lex *lexer) stateFn {
	if !lex.acceptRun(rfc7230TChar) {
		return lex.errorf(ctx, "expected value")
	}

	lex.emit(ctx, "value")

	return lexAfterChallengeBody
}

func lexAfterChallengeBody(ctx context.Context, lex *lexer) stateFn {
	lex.acceptRun(unicode.Space)
	lex.ignore()

	if lex.eof() {
		return nil
	}

	if lex.peek() == ',' {
		return handleCommaAfterChallengeBody(lex)
	}

	return lex.errorf(ctx, "unexpected character %q", lex.peek())
}

func handleCommaAfterChallengeBody(lex *lexer) stateFn {
	lex.next()
	lex.ignore()

	lex.acceptRun(unicode.Space)
	lex.ignore()

	if lex.eof() {
		return nil // trailing comma
	}

	// Distinguish next param vs next challenge
	savePos := lex.pos
	lex.acceptRun(rfc7230TChar)
	lex.acceptRun(unicode.Space)

	if lex.peek() == '=' {
		// It's a key of next param
		lex.pos = savePos

		return lexParam
	}

	// It's a new challenge
	lex.pos = savePos

	return lexChallenge
}

func lexParam(ctx context.Context, lex *lexer) stateFn {
	if !lex.acceptRun(rfc7230TChar) {
		return lex.errorf(ctx, "expected param key")
	}

	lex.emit(ctx, "key")
	lex.acceptRun(unicode.Space)

	if lex.peek() != '=' {
		return lex.errorf(ctx, "expected = after key")
	}

	lex.next()
	lex.ignore()

	return lexValue
}

func emitQuotedString(ctx context.Context, lex *lexer, typ string) error {
	if lex.next() != '"' {
		return ErrMissedOpeningQuote
	}

	lex.ignore()

	var builder strings.Builder

	return lexQuotedLoop(ctx, lex, &builder, typ)
}

func lexQuotedLoop(ctx context.Context, lex *lexer, builder *strings.Builder, typ string) error {
	for {
		runeVal := lex.next()
		if runeVal == -1 {
			return ErrUnclosedQuote
		}

		if runeVal == '\\' {
			runeVal = lex.next()
			if runeVal == -1 {
				return ErrUnexpectedEOF
			}

			builder.WriteRune(runeVal)

			continue
		}

		if runeVal == '"' {
			lex.emitValue(ctx, typ, builder.String())
			lex.ignore()

			return nil
		}

		builder.WriteRune(runeVal)
	}
}
