package ast

import (
	"fmt"
	"strings"

	"inox/compiler/diagnostic"
)

type Type string

const (
	U8  Type = "u8"
	U16 Type = "u16"
	U32 Type = "u32"
	U64 Type = "u64"
)

func (t Type) Bits() int {
	switch t {
	case U8:
		return 8
	case U16:
		return 16
	case U32:
		return 32
	case U64:
		return 64
	default:
		return 0
	}
}

type Program struct {
	Statements []Statement
}

type Statement interface {
	Position() diagnostic.Position
	isStatement()
}

type RegisterDecl struct {
	Name        string
	Type        Type
	Initializer *uint64
	Pin         string
	PinPos      diagnostic.Position
	Pos         diagnostic.Position
}

func (s RegisterDecl) Position() diagnostic.Position { return s.Pos }
func (RegisterDecl) isStatement()                    {}

type Label struct {
	Name string
	Pos  diagnostic.Position
}

func (s Label) Position() diagnostic.Position { return s.Pos }
func (Label) isStatement()                    {}

type Opcode string

const (
	Move    Opcode = "move"
	Add     Opcode = "add"
	Sub     Opcode = "sub"
	Compare Opcode = "compare"
	Jump    Opcode = "jump"
	JumpIf  Opcode = "jump_if"
	Halt    Opcode = "halt"
)

type Operand struct {
	Register  string
	Immediate *uint64
	Pos       diagnostic.Position
}

func (o Operand) IsRegister() bool { return o.Register != "" }

type Instruction struct {
	Op        Opcode
	Left      Operand
	Right     Operand
	Condition string
	Target    string
	Pos       diagnostic.Position
}

func (s Instruction) Position() diagnostic.Position { return s.Pos }
func (Instruction) isStatement()                    {}

func Format(program *Program) string {
	var out strings.Builder
	for _, statement := range program.Statements {
		switch s := statement.(type) {
		case RegisterDecl:
			fmt.Fprintf(&out, "register %s: %s", s.Name, s.Type)
			if s.Initializer != nil {
				fmt.Fprintf(&out, " = %d", *s.Initializer)
			}
			if s.Pin != "" {
				fmt.Fprintf(&out, " @%s", s.Pin)
			}
		case Label:
			fmt.Fprintf(&out, "%s:", s.Name)
		case Instruction:
			fmt.Fprintf(&out, "%s", s.Op)
			switch s.Op {
			case Move, Add, Sub, Compare:
				fmt.Fprintf(&out, " %s, %s", formatOperand(s.Left), formatOperand(s.Right))
			case Jump:
				fmt.Fprintf(&out, " %s", s.Target)
			case JumpIf:
				fmt.Fprintf(&out, " %s, %s", s.Condition, s.Target)
			}
		}
		out.WriteByte('\n')
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
