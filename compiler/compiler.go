package compiler

import (
	"strings"

	"inox/backend/boot16"
	"inox/backend/x86_64"
	"inox/compiler/ast"
	"inox/compiler/diagnostic"
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
	Target   string
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
	target := ast.TargetLinuxX8664
	if program.Target != "" {
		target = program.Target
	}
	if target == ast.TargetBoot16 {
		if len(program.Statements) == 0 {
			return nil, diagnostic.New(file, source, diagnostic.Position{Line: 1, Column: 1}, "boot16 target requires at least one asm, bios_print, or stage2 statement")
		}
		for index, statement := range program.Statements {
			switch node := statement.(type) {
			case ast.AssemblyBlock:
			case ast.BIOSPrint:
				if strings.IndexByte(node.Text, 0) >= 0 {
					return nil, diagnostic.New(file, source, node.Pos, "bios_print text cannot contain a NUL byte")
				}
			case ast.Stage2Block:
				if index != len(program.Statements)-1 {
					return nil, diagnostic.New(file, source, node.Pos, "stage2 must be the final statement")
				}
				if strings.TrimSpace(node.Source) == "" {
					return nil, diagnostic.New(file, source, node.Pos, "stage2 block cannot be empty")
				}
			default:
				return nil, diagnostic.New(file, source, statement.Position(), "boot16 currently supports only asm blocks and bios_print statements")
			}
		}
		return &Result{Tokens: tokens, AST: program, Assembly: boot16.Generate(program), Target: target}, nil
	}
	for _, statement := range program.Statements {
		switch statement.(type) {
		case ast.AssemblyBlock, ast.BIOSPrint, ast.Stage2Block:
			return nil, diagnostic.New(file, source, statement.Position(), "asm, bios_print, and stage2 require target boot16")
		}
	}
	irProgram, err := semantic.Analyze(file, source, program)
	if err != nil {
		return nil, err
	}
	return &Result{Tokens: tokens, AST: program, IR: irProgram, Assembly: x86_64.Generate(irProgram), Target: target}, nil
}
