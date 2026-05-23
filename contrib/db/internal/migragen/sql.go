package main

import (
	"strings"
)

type splitState struct {
	inQuote  bool
	inDollar bool
}

func splitSQLStatements(sqlContent string) []string {
	var statements []string

	var current strings.Builder

	state := &splitState{}

	runes := []rune(sqlContent)

	for i := 0; i < len(runes); i++ {
		i = processRune(runes, i, state, &current)

		isEnd := runes[i] == ';' && !state.inQuote && !state.inDollar
		if isEnd {
			current.WriteRune(';')
			statements = appendStatement(statements, current.String())
			current.Reset()
		}
	}

	return appendStatement(statements, current.String())
}

func processRune(runes []rune, i int, state *splitState, current *strings.Builder) int {
	char := runes[i]
	if char == '\'' && !state.inDollar {
		state.inQuote = !state.inQuote
	} else if isDollarBlock(runes, i, char, state) {
		state.inDollar = !state.inDollar

		current.WriteString("$$")

		return i + 1
	}

	if char != ';' || state.inQuote || state.inDollar {
		current.WriteRune(char)
	}

	return i
}

func isDollarBlock(runes []rune, i int, char rune, state *splitState) bool {
	if char != '$' || state.inQuote {
		return false
	}

	return i+1 < len(runes) && runes[i+1] == '$'
}

func appendStatement(statements []string, sqlStr string) []string {
	stmt := strings.TrimSpace(sqlStr)
	if stmt != "" {
		return append(statements, stmt)
	}

	return statements
}
