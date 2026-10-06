package compiler

import (
	"inox/backend/x86_64"
	"inox/compiler/ast"
	"inox/compiler/ir"
	"inox/compiler/lexer"
	"inox/compiler/parser"
	"inox/compiler/semantic"
)

type Result struct {
	Tokens   []lexer.Token
	AST      *ast.Program
	IR       *ir.Program
	Assembly string
}

func Compile(file, source string) (*Result, error) {
	tokens, err := lexer.Lex(file, source)
	if err != nil {
		return nil, err
	}
	program, err := parser.ParseTokens(file, source, tokens)
	if err != nil {
		return nil, err
	}
	irProgram, err := semantic.Analyze(file, source, program)
	if err != nil {
		return nil, err
	}
	return &Result{Tokens: tokens, AST: program, IR: irProgram, Assembly: x86_64.Generate(irProgram)}, nil
}
