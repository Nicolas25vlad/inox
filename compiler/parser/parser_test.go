package parser

import (
	"strings"
	"testing"

	"inox/compiler/ast"
)

func TestParseBuildsASTForMinimumSyntax(t *testing.T) {
	program, err := Parse("basic.ix", "register a: u64 = 5\nregister b: u64 = 10\nadd a, b\ncompare a, b\njump_if equal, finished\nfinished:\nhalt\n")
	if err != nil {
		t.Fatal(err)
	}
	if len(program.Statements) != 7 {
		t.Fatalf("got %d statements, want 7", len(program.Statements))
	}
	first, ok := program.Statements[0].(ast.RegisterDecl)
	if !ok || first.Name != "a" || first.Type != ast.U64 || first.Initializer == nil || *first.Initializer != 5 {
		t.Fatalf("first statement = %#v, want initialized u64 register a", program.Statements[0])
	}
	conditional, ok := program.Statements[4].(ast.Instruction)
	if !ok || conditional.Op != ast.JumpIf || conditional.Condition != "equal" || conditional.Target != "finished" {
		t.Fatalf("conditional statement = %#v", program.Statements[4])
	}
	label, ok := program.Statements[5].(ast.Label)
	if !ok || label.Name != "finished" {
		t.Fatalf("label statement = %#v", program.Statements[5])
	}
}

func TestParseReportsReadableSyntaxError(t *testing.T) {
	_, err := Parse("bad.ix", "register : u64\n")
	if err == nil || err.Error() != "error: expected register name\n --> bad.ix:1:10\n\n1 | register : u64\n  |          ^" {
		t.Fatalf("unexpected diagnostic: %v", err)
	}
}

func TestParsePinnedRegister(t *testing.T) {
	program, err := Parse("pin.ix", "register result: u64 @rax\n")
	if err != nil {
		t.Fatal(err)
	}
	decl := program.Statements[0].(ast.RegisterDecl)
	if decl.Pin != "rax" {
		t.Fatalf("pin = %q, want rax", decl.Pin)
	}
	if decl.PinPos.Line != 1 || decl.PinPos.Column != 23 {
		t.Fatalf("pin location = %d:%d, want 1:23", decl.PinPos.Line, decl.PinPos.Column)
	}
}

func TestParseBoot16TargetAndBIOSStatements(t *testing.T) {
	source := "target boot16\nbios_print \"Inox\\nboot\"\nasm {\n    mov ah, 0x0e\n    int 0x10\n}\nstage2 {\n    mov ax, 0x1234\n}\n"
	program, err := Parse("boot.ix", source)
	if err != nil {
		t.Fatal(err)
	}
	if program.Target != ast.TargetBoot16 {
		t.Fatalf("target = %q, want %q", program.Target, ast.TargetBoot16)
	}
	if len(program.Statements) != 3 {
		t.Fatalf("got %d statements, want 3", len(program.Statements))
	}
	print, ok := program.Statements[0].(ast.BIOSPrint)
	if !ok || print.Text != "Inox\nboot" {
		t.Fatalf("first statement = %#v, want decoded BIOS text", program.Statements[0])
	}
	block, ok := program.Statements[1].(ast.AssemblyBlock)
	if !ok || !strings.Contains(block.Source, "mov ah, 0x0e") || !strings.Contains(block.Source, "int 0x10") {
		t.Fatalf("second statement = %#v, want raw BIOS assembly", program.Statements[1])
	}
	stage2, ok := program.Statements[2].(ast.Stage2Block)
	if !ok || !strings.Contains(stage2.Source, "mov ax, 0x1234") {
		t.Fatalf("third statement = %#v, want second-stage assembly", program.Statements[2])
	}
}

func TestParseRejectsDuplicateAndUnknownTargets(t *testing.T) {
	for _, tc := range []struct{ source, want string }{
		{"target boot16\ntarget boot16\n", "target may only be declared once"},
		{"target arm64\n", "unknown target \"arm64\""},
	} {
		if _, err := Parse("target.ix", tc.source); err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("Parse(%q) error = %v, want containing %q", tc.source, err, tc.want)
		}
	}
}

func TestParseDecimalAndHexNumbers(t *testing.T) {
	program, err := Parse("numbers.ix", "register decimal: u16 = 010\nregister hex: u16 = 0xff\n")
	if err != nil {
		t.Fatal(err)
	}
	decimal := program.Statements[0].(ast.RegisterDecl)
	hexadecimal := program.Statements[1].(ast.RegisterDecl)
	if *decimal.Initializer != 10 || *hexadecimal.Initializer != 255 {
		t.Fatalf("parsed values = %d and %d", *decimal.Initializer, *hexadecimal.Initializer)
	}
}

func TestParseReportsMalformedStatements(t *testing.T) {
	for _, tc := range []struct{ source, want string }{
		{"register a u8\n", "expected ':' after register name"},
		{"register a: u128\n", "unknown register type"},
		{"register a: u8 = 1 = 2\n", "duplicate register initializer"},
		{"register a: u8 @rax @rbx\n", "duplicate register pin"},
		{"move a 1\n", "expected ',' between operands"},
		{"jump\n", "expected label name after 'jump'"},
		{"jump_if equal done\n", "expected ',' after jump condition"},
		{"halt extra\n", "unexpected \"extra\" after statement"},
		{"unknown_op\n", "unknown instruction"},
		{"register a: u64 = 18446744073709551616\n", "invalid u64 integer literal"},
	} {
		_, err := Parse("bad.ix", tc.source)
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("Parse(%q) error = %v, want containing %q", tc.source, err, tc.want)
		}
	}
}
