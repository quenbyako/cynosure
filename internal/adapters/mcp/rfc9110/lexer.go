package rfc9110

import (
	"fmt"
	"iter"
	"unicode"
)

type lexer struct {
	input []rune     // the string being scanned.
	start int        // start position of this item.
	pos   int        // current position in the input.
	items chan token // channel of scanned items.
}

type stateFn func(*lexer) stateFn

func lex(input string, start stateFn) chan token {
	l := &lexer{
		input: []rune(input),
		items: make(chan token),
	}
	go l.run(start) // Concurrently run state machine.
	return l.items
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

func (l *lexer) nextIter() iter.Seq[rune] {
	return func(yield func(rune) bool) {
		for l.pos < len(l.input) {
			if !yield(l.input[l.pos]) {
				return
			}
			l.pos++
		}
	}
}

func (l *lexer) backup() {
	if l.pos > l.start {
		l.pos--
	}
}

func (l *lexer) ignore() { l.start = l.pos }

func (l *lexer) peek() rune {
	r := l.next()
	if r != -1 {
		l.backup()
	}
	return r
}

func (l *lexer) eof() bool { return l.pos >= len(l.input) }

func (l *lexer) accept(valid *unicode.RangeTable) bool {
	r := l.next()
	if unicode.Is(valid, r) {
		return true
	}
	if r != -1 {
		l.backup()
	}
	return false
}

func (l *lexer) acceptRun(valid *unicode.RangeTable) (matched bool) {
	for {
		r := l.next()
		if unicode.Is(valid, r) {
			matched = true
		} else {
			if r != -1 {
				l.backup()
			}
			break
		}
	}
	return matched
}

func (l *lexer) emit(t string) {
	l.items <- token{t, string(l.input[l.start:l.pos])}
	l.start = l.pos
}

func (l *lexer) emitValue(t string, val string) {
	l.items <- token{t, val}
	l.start = l.pos
}

func (l *lexer) errorf(format string, args ...interface{}) stateFn {
	l.items <- token{"error", fmt.Sprintf(format, args...)}
	return nil
}

type token struct{ typ, value string }
