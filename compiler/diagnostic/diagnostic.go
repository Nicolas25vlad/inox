package diagnostic

import (
	"fmt"
	"strings"
)

// Position is a one-based source location. Offset is a zero-based rune offset.
type Position struct {
	Offset int
	Line   int
	Column int
}

// Error is a source error rendered with a file, line, column, and source caret.
type Error struct {
	File    string
	Source  string
	Pos     Position
	Message string
}

func (e *Error) Error() string {
	if e.Pos.Line < 1 {
		return fmt.Sprintf("error: %s", e.Message)
	}

	lines := strings.Split(e.Source, "\n")
	lineText := ""
	if e.Pos.Line <= len(lines) {
		lineText = strings.TrimSuffix(lines[e.Pos.Line-1], "\r")
	}
	width := len(fmt.Sprint(e.Pos.Line))
	caretColumn := e.Pos.Column - 1
	if caretColumn < 0 {
		caretColumn = 0
	}
	return fmt.Sprintf("error: %s\n --> %s:%d:%d\n\n%*d | %s\n%s| %s^",
		e.Message, e.File, e.Pos.Line, e.Pos.Column, width, e.Pos.Line,
		lineText, strings.Repeat(" ", width+1), strings.Repeat(" ", caretColumn))
}

func New(file, source string, pos Position, message string) *Error {
	return &Error{File: file, Source: source, Pos: pos, Message: message}
}
