package parser

import (
	"fmt"
	"strconv"
	"strings"

	"inox/compiler/ast"
	"inox/compiler/diagnostic"
	"inox/compiler/lexer"
)

func Parse(file, source string) (*ast.Program, error) {
	tokens, err := lexer.Lex(file, source)
	if err != nil {
		return nil, err
	}
	return ParseTokens(file, source, tokens)
}

func ParseTokens(file, source string, tokens []lexer.Token) (*ast.Program, error) {
	p := &parser{file: file, source: source, tokens: tokens}
	program := &ast.Program{}
	for p.current().Kind != lexer.TokenEOF {
		if p.match(lexer.TokenNewline) {
			continue
		}
		statement, err := p.parseStatement()
		if err != nil {
			return nil, err
		}
		program.Statements = append(program.Statements, statement)
	}
	return program, nil
}

type parser struct {
	file   string
	source string
	tokens []lexer.Token
	index  int
}

func (p *parser) parseStatement() (ast.Statement, error) {
	start := p.current()
	if start.Kind != lexer.TokenIdentifier {
		return nil, p.errorAt(start, "expected instruction, register declaration, or label")
	}
	p.advance()
	if start.Lexeme == "register" {
		decl, err := p.parseRegister(start)
		if err != nil {
			return nil, err
		}
		if err := p.finishLine(); err != nil {
			return nil, err
		}
		return decl, nil
	}
	if p.match(lexer.TokenColon) {
		if err := p.finishLine(); err != nil {
			return nil, err
		}
		return ast.Label{Name: start.Lexeme, Pos: start.Pos}, nil
	}
	return p.parseInstruction(start)
}

func (p *parser) parseRegister(start lexer.Token) (ast.Statement, error) {
	name, err := p.expect(lexer.TokenIdentifier, "expected register name")
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(lexer.TokenColon, "expected ':' after register name"); err != nil {
		return nil, err
	}
	typeToken, err := p.expect(lexer.TokenIdentifier, "expected register type (u8, u16, u32, or u64)")
	if err != nil {
		return nil, err
	}
	registerType := ast.Type(typeToken.Lexeme)
	if registerType.Bits() == 0 {
		return nil, p.errorAt(typeToken, "unknown register type %q; expected u8, u16, u32, or u64", typeToken.Lexeme)
	}

	decl := ast.RegisterDecl{Name: name.Lexeme, Type: registerType, Pos: start.Pos}
	seenInitializer, seenPin := false, false
	for p.current().Kind == lexer.TokenEqual || p.current().Kind == lexer.TokenAt {
		if p.match(lexer.TokenEqual) {
			if seenInitializer {
				return nil, p.errorAt(p.previous(), "duplicate register initializer")
			}
			seenInitializer = true
			value, err := p.parseInteger("expected integer literal after '='")
			if err != nil {
				return nil, err
			}
			decl.Initializer = &value
			continue
		}
		at := p.advance()
		if seenPin {
			return nil, p.errorAt(at, "duplicate register pin")
		}
		seenPin = true
		pin, err := p.expect(lexer.TokenIdentifier, "expected physical register name after '@'")
		if err != nil {
			return nil, err
		}
		decl.Pin = pin.Lexeme
	}
	return decl, nil
}

func (p *parser) parseInstruction(start lexer.Token) (ast.Statement, error) {
	instruction := ast.Instruction{Op: ast.Opcode(start.Lexeme), Pos: start.Pos}
	switch instruction.Op {
	case ast.Move, ast.Add, ast.Sub, ast.Compare:
		left, err := p.parseOperand()
		if err != nil {
			return nil, err
		}
		if _, err := p.expect(lexer.TokenComma, "expected ',' between operands"); err != nil {
			return nil, err
		}
		right, err := p.parseOperand()
		if err != nil {
			return nil, err
		}
		instruction.Left, instruction.Right = left, right
	case ast.Jump:
		target, err := p.expect(lexer.TokenIdentifier, "expected label name after 'jump'")
		if err != nil {
			return nil, err
		}
		instruction.Target = target.Lexeme
	case ast.JumpIf:
		condition, err := p.expect(lexer.TokenIdentifier, "expected condition after 'jump_if'")
		if err != nil {
			return nil, err
		}
		if _, err := p.expect(lexer.TokenComma, "expected ',' after jump condition"); err != nil {
			return nil, err
		}
		target, err := p.expect(lexer.TokenIdentifier, "expected label name after jump condition")
		if err != nil {
			return nil, err
		}
		instruction.Condition, instruction.Target = condition.Lexeme, target.Lexeme
	case ast.Halt:
	default:
		return nil, p.errorAt(start, "unknown instruction %q", start.Lexeme)
	}
	if err := p.finishLine(); err != nil {
		return nil, err
	}
	return instruction, nil
}

func (p *parser) parseOperand() (ast.Operand, error) {
	token := p.current()
	switch token.Kind {
	case lexer.TokenIdentifier:
		p.advance()
		return ast.Operand{Register: token.Lexeme, Pos: token.Pos}, nil
	case lexer.TokenNumber:
		value, err := p.parseInteger("expected integer literal")
		if err != nil {
			return ast.Operand{}, err
		}
		return ast.Operand{Immediate: &value, Pos: token.Pos}, nil
	default:
		return ast.Operand{}, p.errorAt(token, "expected register name or integer literal")
	}
}

func (p *parser) parseInteger(message string) (uint64, error) {
	token, err := p.expect(lexer.TokenNumber, message)
	if err != nil {
		return 0, err
	}
	literal := strings.ReplaceAll(token.Lexeme, "_", "")
	base := 10
	if strings.HasPrefix(literal, "0x") || strings.HasPrefix(literal, "0X") {
		base = 0
	}
	value, parseErr := strconv.ParseUint(literal, base, 64)
	if parseErr != nil {
		return 0, p.errorAt(token, "invalid u64 integer literal %q", token.Lexeme)
	}
	return value, nil
}

func (p *parser) finishLine() error {
	if p.match(lexer.TokenNewline) || p.current().Kind == lexer.TokenEOF {
		return nil
	}
	return p.errorAt(p.current(), "unexpected %s after statement", describe(p.current()))
}

func (p *parser) expect(kind lexer.TokenKind, message string) (lexer.Token, error) {
	if p.current().Kind != kind {
		return lexer.Token{}, p.errorAt(p.current(), message)
	}
	return p.advance(), nil
}

func (p *parser) match(kind lexer.TokenKind) bool {
	if p.current().Kind != kind {
		return false
	}
	p.advance()
	return true
}

func (p *parser) current() lexer.Token { return p.tokens[p.index] }

func (p *parser) previous() lexer.Token { return p.tokens[p.index-1] }

func (p *parser) advance() lexer.Token {
	token := p.current()
	if token.Kind != lexer.TokenEOF {
		p.index++
	}
	return token
}

func (p *parser) errorAt(token lexer.Token, format string, args ...any) error {
	return diagnostic.New(p.file, p.source, token.Pos, fmt.Sprintf(format, args...))
}

func describe(token lexer.Token) string {
	if token.Kind == lexer.TokenEOF {
		return "end of file"
	}
	return fmt.Sprintf("%q", token.Lexeme)
}
