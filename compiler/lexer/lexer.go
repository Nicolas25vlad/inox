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
	TokenString
	TokenAssemblyBlock
	TokenStage2Block
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
	case TokenString:
		return "string"
	case TokenAssemblyBlock:
		return "assembly block"
	case TokenStage2Block:
		return "stage2 block"
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
		if char == '"' {
			pos := diagnostic.Position{Offset: i, Line: line, Column: column}
			start := i
			i++
			column++
			closed := false
			for i < len(runes) {
				if runes[i] == '\n' {
					return nil, diagnostic.New(file, source, pos, "unterminated string literal")
				}
				if runes[i] == '\\' {
					i++
					column++
					if i < len(runes) {
						i++
						column++
					}
					continue
				}
				if runes[i] == '"' {
					i++
					column++
					closed = true
					break
				}
				i++
				column++
			}
			if !closed {
				return nil, diagnostic.New(file, source, pos, "unterminated string literal")
			}
			tokens = append(tokens, Token{Kind: TokenString, Lexeme: string(runes[start:i]), Pos: pos})
			continue
		}

		pos := diagnostic.Position{Offset: i, Line: line, Column: column}
		if isIdentifierStart(char) {
			start := i
			for i < len(runes) && isIdentifierPart(runes[i]) {
				i++
				column++
			}
			lexeme := string(runes[start:i])
			tokens = append(tokens, Token{Kind: TokenIdentifier, Lexeme: lexeme, Pos: pos})
			if lexeme == "asm" || lexeme == "stage2" {
				open := i
				openColumn := column
				for open < len(runes) && (runes[open] == ' ' || runes[open] == '\t') {
					open++
					openColumn++
				}
				if open < len(runes) && runes[open] == '{' {
					body, end, nextLine, nextColumn, err := scanAssemblyBlock(file, source, runes, open, line, openColumn)
					if err != nil {
						return nil, err
					}
					kind := TokenAssemblyBlock
					if lexeme == "stage2" {
						kind = TokenStage2Block
					}
					tokens = append(tokens, Token{
						Kind: kind, Lexeme: body,
						Pos: diagnostic.Position{Offset: open, Line: line, Column: openColumn},
					})
					i, line, column = end, nextLine, nextColumn
				}
			}
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

func scanAssemblyBlock(file, source string, runes []rune, open, line, column int) (string, int, int, int, error) {
	start := open + 1
	i := start
	depth := 1
	quote := rune(0)
	advance := func() {
		if runes[i] == '\n' {
			line++
			column = 1
		} else {
			column++
		}
		i++
	}
	column++
	for i < len(runes) {
		char := runes[i]
		if quote != 0 {
			if char == '\\' && i+1 < len(runes) {
				advance()
				advance()
				continue
			}
			if char == quote {
				quote = 0
			}
			advance()
			continue
		}
		if char == ';' {
			for i < len(runes) && runes[i] != '\n' {
				advance()
			}
			continue
		}
		if char == '\'' || char == '"' {
			quote = char
			advance()
			continue
		}
		if char == '{' {
			depth++
		} else if char == '}' {
			depth--
			if depth == 0 {
				return string(runes[start:i]), i + 1, line, column + 1, nil
			}
		}
		advance()
	}
	position := diagnostic.Position{Offset: len(runes), Line: line, Column: column}
	return "", 0, 0, 0, diagnostic.New(file, source, position, "expected '}' to close asm block")
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
