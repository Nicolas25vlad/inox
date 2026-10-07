package compiler

import (
	"strings"
	"testing"

	"inox/compiler/ast"
)

func TestCompileRunsTheFullPipeline(t *testing.T) {
	result, err := Compile("hello.ix", "register value: u64 = 42\nhalt\n")
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Tokens) == 0 || result.AST == nil || result.IR == nil || !strings.Contains(result.Assembly, "mov rax, 42") {
		t.Fatalf("incomplete compiler result: %#v", result)
	}
}

func TestCompileGeneratesBootableBIOSAssembly(t *testing.T) {
	result, err := Compile("boot.ix", "target boot16\nbios_print \"Inox\"\nasm {\n    cli\n}\nstage2 {\n    mov ax, 0x1234\n}\n")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"bits 16", "org 0x7c00", "int 0x10", "mov word [__inox_remaining]", "int 0x13", "__inox_stage2_start:", "mov ax, 0x1234", "__inox_text_0: db 0x49, 0x6e, 0x6f, 0x78, 0", "times 510 - ($ - $$) db 0", "dw 0xaa55", "times 1474560 - ($ - $$) db 0"} {
		if !strings.Contains(result.Assembly, want) {
			t.Errorf("boot assembly missing %q:\n%s", want, result.Assembly)
		}
	}
	if result.Target != ast.TargetBoot16 {
		t.Fatalf("target = %q, want %q", result.Target, ast.TargetBoot16)
	}
}

func TestCompileRejectsUnsupportedStatementsForBoot16(t *testing.T) {
	for _, source := range []string{
		"target boot16\nregister value: u16\n",
		"asm { mov ax, 0 }\n",
		"target boot16\n",
		"target boot16\nstage2 { }\n",
		"target boot16\nstage2 { nop }\nbios_print \"late\"\n",
	} {
		if _, err := Compile("bad-target.ix", source); err == nil {
			t.Errorf("Compile(%q) succeeded, want target diagnostic", source)
		}
	}
}

func TestCompileReturnsLexerParserAndSemanticErrors(t *testing.T) {
	for _, source := range []string{
		"halt\n$\n",
		"register : u64\n",
		"register a: u8\nregister a: u8\n",
	} {
		if _, err := Compile("bad.ix", source); err == nil || !strings.Contains(err.Error(), "error:") {
			t.Errorf("Compile(%q) error = %v, want a source error", source, err)
		}
	}
}
