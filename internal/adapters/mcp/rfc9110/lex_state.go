package rfc9110

import (
	"errors"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/rangetable"
)

var (
	alphanum = rangetable.New([]rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")...)
	// tchar as defined in RFC 9110 Section 5.6.2
	rfc7230TChar = rangetable.Merge(alphanum, rangetable.New([]rune("!#$%&'*+-.^_`|~+")...))
	// token68 as defined in RFC 9110 Section 11.2
	base68 = rangetable.Merge(alphanum, rangetable.New([]rune("-._~+/")...))

	tokensBoth = rangetable.Merge(rfc7230TChar, base68)

	equal = rangetable.New('=')
	comma = rangetable.New(',')
)

func lexChallenge(l *lexer) stateFn {
	l.acceptRun(unicode.Space)
	l.ignore()

	// Skip empty list elements (leading or between challenges)
	for l.peek() == ',' {
		l.next()
		l.ignore()
		l.acceptRun(unicode.Space)
		l.ignore()
	}

	if l.eof() {
		return nil
	}

	if !l.acceptRun(rfc7230TChar) {
		return l.errorf("invalid format, challenge must start with token")
	}
	l.emit("auth-scheme")

	return lexAfterScheme
}

func lexAfterScheme(l *lexer) stateFn {
	hasSpace := l.acceptRun(unicode.Space)
	l.ignore()

	if l.eof() {
		return nil
	}
	if l.peek() == ',' {
		l.next()
		l.ignore()
		return lexChallenge
	}

	if !hasSpace {
		return l.errorf("expected space after auth-scheme")
	}

	// It's either a token68 or the start of auth-params
	// We read characters that could be either.
	// Note: rfc7230TChar excludes '/', but base68 includes it.
	l.acceptRun(tokensBoth)
	tokPos := l.pos

	l.acceptRun(unicode.Space)
	if l.peek() == '=' {
		// If next is '=', check if it's double '==' (token68 padding)
		l.next()
		if l.peek() == '=' {
			// It's token68 padding. RFC 9110 Section 11.2: token68 ends with *"="
			l.next()
			l.acceptRun(equal)
			l.emit("token68")
			return lexAfterChallengeBody
		}
		// It was a single '=', so the token we read is a key OR token68 with padding.
		// If it's followed by a value (token or quoted-string), it's a key.
		// If it's followed by a comma or EOF, it's a token68.
		l.acceptRun(unicode.Space)
		p := l.peek()
		if p == ',' || p == -1 {
			// token68 with single padding
			l.emit("token68")
			return lexAfterChallengeBody
		}

		// Also check if it's clearly a token68 by containing '/'
		tokenVal := string(l.input[l.start:tokPos])
		if !strings.Contains(tokenVal, "/") && (unicode.Is(rfc7230TChar, p) || p == '"') {
			l.pos = tokPos
			l.emit("key")
			l.acceptRun(unicode.Space)
			l.next() // consume the '='
			l.ignore()
			return lexValue
		}

		// Fallback to token68
		l.emit("token68")
		return lexAfterChallengeBody
	}

	// If no '=', it might be a token68 that doesn't use '=' or ends here.
	// We need to re-read it as base68 to be sure it's valid.
	// But according to RFC, if it's not a key, it must be token68.
	l.pos = tokPos
	// Optionally check base68 validity here if strictness is needed
	l.emit("token68")
	return lexAfterChallengeBody
}

func lexValue(l *lexer) stateFn {
	l.acceptRun(unicode.Space)
	l.ignore()

	if l.peek() == '"' {
		if err := emitQuotedString(l, "value"); err != nil {
			return l.errorf("%v", err)
		}
	} else {
		if !l.acceptRun(rfc7230TChar) {
			return l.errorf("expected value")
		}
		l.emit("value")
	}
	return lexAfterChallengeBody
}

func lexAfterChallengeBody(l *lexer) stateFn {
	l.acceptRun(unicode.Space)
	l.ignore()

	if l.eof() {
		return nil
	}

	if l.peek() == ',' {
		l.next()
		l.ignore()

		l.acceptRun(unicode.Space)
		l.ignore()

		if l.eof() {
			return nil // trailing comma
		}

		// Distinguish next param vs next challenge
		savePos := l.pos
		l.acceptRun(rfc7230TChar)
		l.acceptRun(unicode.Space)
		if l.peek() == '=' {
			// It's a key of next param
			l.pos = savePos
			return lexParam
		}

		// It's a new challenge
		l.pos = savePos
		return lexChallenge
	}

	return l.errorf("unexpected character %q", l.peek())
}

func lexParam(l *lexer) stateFn {
	if !l.acceptRun(rfc7230TChar) {
		return l.errorf("expected param key")
	}
	l.emit("key")
	l.acceptRun(unicode.Space)
	if l.peek() != '=' {
		return l.errorf("expected = after key")
	}
	l.next()
	l.ignore()
	return lexValue
}

func emitQuotedString(l *lexer, typ string) error {
	if l.next() != '"' {
		return errors.New("missed opening quote")
	}
	l.ignore()

	var b strings.Builder
	for {
		r := l.next()
		if r == -1 {
			return errors.New("unclosed quote")
		}
		if r == '\\' {
			r = l.next()
			if r == -1 {
				return errors.New("unexpected EOF")
			}
			b.WriteRune(r)
			continue
		}
		if r == '"' {
			l.emitValue(typ, b.String())
			l.ignore()
			return nil
		}
		b.WriteRune(r)
	}
}
