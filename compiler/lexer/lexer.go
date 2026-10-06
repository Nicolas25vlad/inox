package lexer

import (
	"fmt"
	"strings"

	"inox/compiler/diagnostic"
)

type TokenKind uint8

const (
	TokenEOF TokenKind = iota
	TokenNewline
	TokenIdentifier
	TokenNumber
	TokenColon
	TokenComma
	TokenEqual
	TokenAt
)

func (k TokenKind) String() string {
	switch k {
	case TokenEOF:
		return "end of file"
	case TokenNewline:
		return "newline"
	case TokenIdentifier:
		return "identifier"
	case TokenNumber:
		return "integer"
	case TokenColon:
		return ":"
	case TokenComma:
		return ","
	case TokenEqual:
		return "="
	case TokenAt:
		return "@"
	default:
		return "unknown"
	}
}

type Token struct {
	Kind   TokenKind
	Lexeme string
	Pos    diagnostic.Position
}

func Lex(file, source string) ([]Token, error) {
	runes := []rune(source)
	tokens := make([]Token, 0, len(runes)/2)
	line, column := 1, 1

	for i := 0; i < len(runes); {
		char := runes[i]
		if char == ' ' || char == '\t' || char == '\r' {
			i++
			column++
			continue
		}
		if char == '\n' {
			tokens = append(tokens, Token{Kind: TokenNewline, Lexeme: "\n", Pos: diagnostic.Position{Offset: i, Line: line, Column: column}})
			i++
			line++
			column = 1
			continue
		}
		if char == '#' || (char == '/' && i+1 < len(runes) && runes[i+1] == '/') {
			for i < len(runes) && runes[i] != '\n' {
				i++
				column++
			}
			continue
		}

		pos := diagnostic.Position{Offset: i, Line: line, Column: column}
		if isIdentifierStart(char) {
			start := i
			for i < len(runes) && isIdentifierPart(runes[i]) {
				i++
				column++
			}
			tokens = append(tokens, Token{Kind: TokenIdentifier, Lexeme: string(runes[start:i]), Pos: pos})
			continue
		}
		if isDigit(char) {
			start := i
			for i < len(runes) && (isLetter(runes[i]) || isDigit(runes[i]) || runes[i] == '_') {
				i++
				column++
			}
			tokens = append(tokens, Token{Kind: TokenNumber, Lexeme: string(runes[start:i]), Pos: pos})
			continue
		}

		kind := TokenKind(255)
		switch char {
		case ':':
			kind = TokenColon
		case ',':
			kind = TokenComma
		case '=':
			kind = TokenEqual
		case '@':
			kind = TokenAt
		}
		if kind == 255 {
			return nil, diagnostic.New(file, source, pos, fmt.Sprintf("unexpected character %q", char))
		}
		tokens = append(tokens, Token{Kind: kind, Lexeme: string(char), Pos: pos})
		i++
		column++
	}

	tokens = append(tokens, Token{Kind: TokenEOF, Pos: diagnostic.Position{Offset: len(runes), Line: line, Column: column}})
	return tokens, nil
}

func isIdentifierStart(char rune) bool { return char == '_' || isLetter(char) }
func isIdentifierPart(char rune) bool  { return char == '_' || isLetter(char) || isDigit(char) }
func isLetter(char rune) bool          { return char >= 'a' && char <= 'z' || char >= 'A' && char <= 'Z' }
func isDigit(char rune) bool           { return char >= '0' && char <= '9' }

func Format(tokens []Token) string {
	var out strings.Builder
	for _, token := range tokens {
		if token.Kind == TokenEOF {
			fmt.Fprintf(&out, "%d:%d %s\n", token.Pos.Line, token.Pos.Column, token.Kind)
			continue
		}
		fmt.Fprintf(&out, "%d:%d %-10s %q\n", token.Pos.Line, token.Pos.Column, token.Kind, token.Lexeme)
	}
	return out.String()
}
