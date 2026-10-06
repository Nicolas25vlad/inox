package semantic

import (
	"fmt"
	"strings"
	"testing"

	"inox/compiler/ast"
	"inox/compiler/parser"
)

func TestAnalyzeAllocatesRegistersAndLowersInstructions(t *testing.T) {
	program, err := parser.Parse("basic.ix", "register a: u64 = 5\nregister b: u64 = 10\nadd a, b\ncompare a, b\njump_if equal, done\ndone:\nhalt\n")
	if err != nil {
		t.Fatal(err)
	}
	irProgram, err := Analyze("basic.ix", "register a: u64 = 5\nregister b: u64 = 10\nadd a, b\ncompare a, b\njump_if equal, done\ndone:\nhalt\n", program)
	if err != nil {
		t.Fatal(err)
	}
	if len(irProgram.Registers) != 2 || irProgram.Registers[0].Physical != "rax" || irProgram.Registers[1].Physical != "rbx" {
		t.Fatalf("unexpected register allocation: %#v", irProgram.Registers)
	}
	if len(irProgram.Instructions) != 7 {
		t.Fatalf("got %d IR instructions, want 7", len(irProgram.Instructions))
	}
}

func TestAnalyzeRejectsUndefinedRegistersAndInvalidDeclarations(t *testing.T) {
	for _, tc := range []struct{ source, want string }{
		{"add missing, 1\nhalt\n", "undefined register 'missing'"},
		{"jump nowhere\n", "undefined label 'nowhere'"},
		{"register a: u8\nregister a: u8\n", "duplicate register 'a'"},
		{"done:\ndone:\n", "duplicate label 'done'"},
		{"register a: u8\nregister b: u16\nadd a, b\n", "register types do not match"},
		{"move 1, 2\n", "destination of 'move' must be a register"},
		{"compare 1, 2\n", "left operand of 'compare' must be a register"},
		{"register a: u8\njump_if invalid, done\ndone:\n", "unknown jump condition"},
		{"add a, 1\nregister a: u8\n", "used before its declaration"},
	} {
		program, err := parser.Parse("bad.ix", tc.source)
		if err != nil {
			t.Fatal(err)
		}
		_, err = Analyze("bad.ix", tc.source, program)
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("Analyze(%q) error = %v, want containing %q", tc.source, err, tc.want)
		}
	}
}

func TestAnalyzeHonorsPinsBeforeAllocatingRemainingRegisters(t *testing.T) {
	source := "register automatic: u64\nregister pinned: u64 @rax\nregister byte: u8 @r8\nmove byte, 5\nhalt\n"
	program, err := parser.Parse("pins.ix", source)
	if err != nil {
		t.Fatal(err)
	}
	irProgram, err := Analyze("pins.ix", source, program)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"rbx", "rax", "r8"}
	if len(irProgram.Registers) != len(want) {
		t.Fatalf("got %d registers, want %d", len(irProgram.Registers), len(want))
	}
	for i, physical := range want {
		if got := irProgram.Registers[i].Physical; got != physical {
			t.Errorf("register %s allocated to %s, want %s", irProgram.Registers[i].Name, got, physical)
		}
	}
	if got := irProgram.Instructions[0].Left.Register; got != "r8b" {
		t.Fatalf("u8 pin lowered to %q, want r8b", got)
	}
}

func TestAnalyzeRejectsDuplicateAndReservedPins(t *testing.T) {
	for _, tc := range []struct{ source, want string }{
		{"register a: u64 @rax\nregister b: u64 @rax\n", "physical register 'rax' is pinned more than once"},
		{"register stack: u64 @rsp\n", "'rsp' is reserved for the stack pointer"},
		{"register scratch: u64 @r11\n", "'r11' is reserved by the x86-64 backend"},
		{"register alias: u64 @eax\n", "unknown physical register 'eax'"},
	} {
		program, err := parser.Parse("bad-pin.ix", tc.source)
		if err != nil {
			t.Fatal(err)
		}
		_, err = Analyze("bad-pin.ix", tc.source, program)
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("Analyze(%q) error = %v, want containing %q", tc.source, err, tc.want)
		}
	}
}

func TestAnalyzeUsesAvailableGeneralPurposeRegisters(t *testing.T) {
	var source strings.Builder
	for i := 0; i < 14; i++ {
		fmt.Fprintf(&source, "register r%d: u64\n", i)
	}
	program, err := parser.Parse("many.ix", source.String())
	if err != nil {
		t.Fatal(err)
	}
	irProgram, err := Analyze("many.ix", source.String(), program)
	if err != nil {
		t.Fatal(err)
	}
	if got := irProgram.Registers[13].Physical; got != "r15" {
		t.Fatalf("last allocated register = %q, want r15", got)
	}
	source.WriteString("register overflow: u64\n")
	program, err = parser.Parse("many.ix", source.String())
	if err != nil {
		t.Fatal(err)
	}
	_, err = Analyze("many.ix", source.String(), program)
	if err == nil || !strings.Contains(err.Error(), "no physical registers available") {
		t.Fatalf("unexpected allocation error: %v", err)
	}
}

func TestRegisterAliasesCoverEveryAllocatableRegisterWidth(t *testing.T) {
	for _, physical := range physicalRegisters {
		for _, registerType := range []ast.Type{ast.U8, ast.U16, ast.U32, ast.U64} {
			if alias := registerAlias(physical, registerType); alias == "" {
				t.Errorf("missing %s alias for %s", registerType, physical)
			}
		}
	}
}

func TestAnalyzeRejectsOutOfRangeImmediate(t *testing.T) {
	source := "register small: u8 = 256\nhalt\n"
	program, err := parser.Parse("range.ix", source)
	if err != nil {
		t.Fatal(err)
	}
	_, err = Analyze("range.ix", source, program)
	if err == nil || !strings.Contains(err.Error(), "does not fit in u8") {
		t.Fatalf("unexpected error: %v", err)
	}
}
