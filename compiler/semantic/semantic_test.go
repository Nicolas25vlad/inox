package semantic

import (
	"strings"
	"testing"

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

func TestAnalyzeRejectsUndefinedRegisterAndUnsupportedPinning(t *testing.T) {
	for _, tc := range []struct{ source, want string }{
		{"add missing, 1\nhalt\n", "undefined register 'missing'"},
		{"register pinned: u64 @rax\nhalt\n", "register pinning is not supported in v0.1"},
		{"register a: u8\nregister b: u8\nregister c: u8\nregister d: u8\nregister e: u8\nhalt\n", "supports up to 4 virtual registers"},
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
