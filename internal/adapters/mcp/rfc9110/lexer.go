package rfc9110

import (
	"context"
	"fmt"
	"unicode"
)

type lexer struct {
	items chan token // channel of scanned items.
	input []rune     // the string being scanned.
	start int        // start position of this item.
	pos   int        // current position in the input.
}

type stateFn func(context.Context, *lexer) stateFn

func lex(ctx context.Context, input string, start stateFn) chan token {
	lex := &lexer{
		input: []rune(input),
		items: make(chan token),
		start: 0,
		pos:   0,
	}
	go lex.run(ctx, start) // Concurrently run state machine.

	return lex.items
}

func (l *lexer) run(ctx context.Context, start stateFn) {
	defer close(l.items)

	for state := start; state != nil; {
		select {
		case <-ctx.Done():
			return
		default:
			state = state(ctx, l)
		}
	}
}

func (l *lexer) next() rune {
	if l.pos >= len(l.input) {
		return -1
	}

	r := l.input[l.pos]
	l.pos++

	return r
}

func (l *lexer) backup() {
	if l.pos > l.start {
		l.pos--
	}
}

func (l *lexer) ignore() {
	l.start = l.pos
}

func (l *lexer) peek() rune {
	r := l.next()
	if r != -1 {
		l.backup()
	}

	return r
}

func (l *lexer) eof() bool {
	return l.pos >= len(l.input)
}

func (l *lexer) acceptRun(valid *unicode.RangeTable) (matched bool) {
	for unicode.Is(valid, l.peek()) {
		l.next()

		matched = true
	}

	return matched
}

func (l *lexer) emit(ctx context.Context, typ string) {
	select {
	case <-ctx.Done():
		return
	case l.items <- token{typ, string(l.input[l.start:l.pos])}:
		l.start = l.pos
	}
}

func (l *lexer) emitValue(ctx context.Context, typ, val string) {
	select {
	case <-ctx.Done():
		return
	case l.items <- token{typ, val}:
		l.start = l.pos
	}
}

func (l *lexer) errorf(ctx context.Context, format string, args ...interface{}) stateFn {
	select {
	case <-ctx.Done():
		return nil
	case l.items <- token{"error", fmt.Sprintf(format, args...)}:
		return nil
	}
}

type token struct{ typ, value string }
