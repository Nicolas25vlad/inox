package ir

import (
	"testing"

	"inox/compiler/ast"
)

func TestProgramStringFormatsIRInstructions(t *testing.T) {
	value := uint64(5)
	program := &Program{
		Registers: []Register{{Name: "a", Type: ast.U64, Physical: "rax"}},
		Instructions: []Instruction{
			{Op: OpMove, Left: Operand{Register: "rax"}, Right: Operand{Immediate: &value}},
			{Op: OpAdd, Left: Operand{Register: "rax"}, Right: Operand{Register: "rbx"}},
			{Op: OpJump, Label: "done"},
			{Op: OpJumpIf, Condition: "equal", Label: "done"},
			{Op: OpLabel, Label: "done"},
			{Op: OpHalt},
		},
	}
	want := "reg a:u64 -> rax\nmove rax, 5\nadd rax, rbx\njump done\njump_if equal done\ndone:\nhalt\n"
	if got := program.String(); got != want {
		t.Fatalf("String() = %q, want %q", got, want)
	}
}
