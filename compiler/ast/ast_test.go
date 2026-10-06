package ast

import (
	"testing"
)

func TestTypesAndFormat(t *testing.T) {
	for _, tc := range []struct {
		typ  Type
		bits int
	}{{U8, 8}, {U16, 16}, {U32, 32}, {U64, 64}, {Type("bad"), 0}} {
		if got := tc.typ.Bits(); got != tc.bits {
			t.Errorf("%s.Bits() = %d, want %d", tc.typ, got, tc.bits)
		}
	}
	value := uint64(7)
	program := &Program{Statements: []Statement{
		RegisterDecl{Name: "a", Type: U8, Initializer: &value, Pin: "al"},
		RegisterDecl{Name: "b", Type: U64},
		Label{Name: "done"},
		Instruction{Op: Move, Left: Operand{Register: "a"}, Right: Operand{Register: "b"}},
		Instruction{Op: Add, Left: Operand{Immediate: &value}, Right: Operand{Immediate: &value}},
		Instruction{Op: Sub, Right: Operand{Register: "a"}},
		Instruction{Op: Compare, Left: Operand{Register: "a"}, Right: Operand{Immediate: &value}},
		Instruction{Op: Jump, Target: "done"},
		Instruction{Op: JumpIf, Condition: "zero", Target: "done"},
		Instruction{Op: Halt},
	}}
	want := "register a: u8 = 7 @al\nregister b: u64\ndone:\nmove a, b\nadd 7, 7\nsub <invalid>, a\ncompare a, 7\njump done\njump_if zero, done\nhalt\n"
	if got := Format(program); got != want {
		t.Fatalf("Format() = %q, want %q", got, want)
	}
}
