package rfc9110

import (
	"fmt"
	"unicode"
)

type lexer struct {
	items chan token // channel of scanned items.
	input []rune     // the string being scanned.
	start int        // start position of this item.
	pos   int        // current position in the input.
}

type stateFn func(*lexer) stateFn

func lex(input string, start stateFn) chan token {
	lex := &lexer{
		input: []rune(input),
		items: make(chan token),
		start: 0,
		pos:   0,
	}
	go lex.run(start) // Concurrently run state machine.

	return lex.items
}

func (l *lexer) run(start stateFn) {
	for state := start; state != nil; {
		state = state(l)
	}

	close(l.items) // No more tokens will be delivered.
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

func (l *lexer) emit(typ string) {
	l.items <- token{typ, string(l.input[l.start:l.pos])}

	l.start = l.pos
}

func (l *lexer) emitValue(typ, val string) {
	l.items <- token{typ, val}

	l.start = l.pos
}

func (l *lexer) errorf(format string, args ...interface{}) stateFn {
	l.items <- token{"error", fmt.Sprintf(format, args...)}

	return nil
}

type token struct{ typ, value string }
