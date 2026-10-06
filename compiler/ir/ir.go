package ir

import (
	"fmt"
	"strings"

	"inox/compiler/ast"
	"inox/compiler/diagnostic"
)

type Op string

const (
	OpMove    Op = "move"
	OpAdd     Op = "add"
	OpSub     Op = "sub"
	OpCompare Op = "compare"
	OpJump    Op = "jump"
	OpJumpIf  Op = "jump_if"
	OpLabel   Op = "label"
	OpHalt    Op = "halt"
)

type Register struct {
	Name     string
	Type     ast.Type
	Physical string
	Pos      diagnostic.Position
}

type Operand struct {
	Register  string
	Type      ast.Type
	Immediate *uint64
}

func (o Operand) IsRegister() bool { return o.Register != "" }

type Instruction struct {
	Op        Op
	Left      Operand
	Right     Operand
	Condition string
	Label     string
	Pos       diagnostic.Position
}

type Program struct {
	Registers    []Register
	Instructions []Instruction
}

func (p *Program) String() string {
	var out strings.Builder
	for _, register := range p.Registers {
		fmt.Fprintf(&out, "reg %s:%s -> %s\n", register.Name, register.Type, register.Physical)
	}
	for _, instruction := range p.Instructions {
		switch instruction.Op {
		case OpLabel:
			fmt.Fprintf(&out, "%s:\n", instruction.Label)
		case OpJump:
			fmt.Fprintf(&out, "jump %s\n", instruction.Label)
		case OpJumpIf:
			fmt.Fprintf(&out, "jump_if %s %s\n", instruction.Condition, instruction.Label)
		case OpHalt:
			out.WriteString("halt\n")
		default:
			fmt.Fprintf(&out, "%s %s, %s\n", instruction.Op, formatOperand(instruction.Left), formatOperand(instruction.Right))
		}
	}
	return out.String()
}

func formatOperand(operand Operand) string {
	if operand.IsRegister() {
		return operand.Register
	}
	if operand.Immediate != nil {
		return fmt.Sprint(*operand.Immediate)
	}
	return "<invalid>"
}
