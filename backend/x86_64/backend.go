package x86_64

import (
	"fmt"
	"strings"

	"inox/compiler/ir"
)

var conditionInstructions = map[string]string{
	"equal": "je", "not_equal": "jne", "zero": "jz", "not_zero": "jnz",
	"less": "jb", "less_equal": "jbe", "greater": "ja", "greater_equal": "jae",
}

func Generate(program *ir.Program) string {
	var out strings.Builder
	out.WriteString("bits 64\ndefault rel\nglobal _start\n\nsection .text\n_start:\n")
	for _, instruction := range program.Instructions {
		switch instruction.Op {
		case ir.OpLabel:
			fmt.Fprintf(&out, "%s:\n", labelName(instruction.Label))
		case ir.OpMove:
			fmt.Fprintf(&out, "    mov %s, %s\n", instruction.Left.Register, operandText(instruction.Right))
		case ir.OpAdd, ir.OpSub, ir.OpCompare:
			mnemonic := string(instruction.Op)
			if instruction.Op == ir.OpCompare {
				mnemonic = "cmp"
			}
			if usesScratch(instruction) {
				fmt.Fprintf(&out, "    mov r11, %s\n", operandText(instruction.Right))
				fmt.Fprintf(&out, "    %s %s, r11\n", mnemonic, instruction.Left.Register)
			} else {
				fmt.Fprintf(&out, "    %s %s, %s\n", mnemonic, instruction.Left.Register, operandText(instruction.Right))
			}
		case ir.OpJump:
			fmt.Fprintf(&out, "    jmp %s\n", labelName(instruction.Label))
		case ir.OpJumpIf:
			fmt.Fprintf(&out, "    %s %s\n", conditionInstructions[instruction.Condition], labelName(instruction.Label))
		case ir.OpHalt:
			writeExit(&out)
		}
	}
	if len(program.Instructions) == 0 || program.Instructions[len(program.Instructions)-1].Op != ir.OpHalt {
		writeExit(&out)
	}
	out.WriteString("\nsection .note.GNU-stack noalloc noexec nowrite progbits\n")
	return out.String()
}

func labelName(sourceName string) string { return "inox_l_" + sourceName }

func usesScratch(instruction ir.Instruction) bool {
	if instruction.Right.Immediate == nil || instruction.Left.Type.Bits() != 64 {
		return false
	}
	return *instruction.Right.Immediate > 1<<31-1
}

func operandText(operand ir.Operand) string {
	if operand.IsRegister() {
		return operand.Register
	}
	if operand.Immediate != nil {
		return fmt.Sprint(*operand.Immediate)
	}
	return "<invalid>"
}

func writeExit(out *strings.Builder) {
	out.WriteString("    mov eax, 60\n    xor edi, edi\n    syscall\n")
}
