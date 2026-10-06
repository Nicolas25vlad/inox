package semantic

import (
	"fmt"

	"inox/compiler/ast"
	"inox/compiler/diagnostic"
	"inox/compiler/ir"
)

var physicalRegisters = []string{"rax", "rbx", "rcx", "rdx"}

var registerAliases = map[string]map[ast.Type]string{
	"rax": {ast.U8: "al", ast.U16: "ax", ast.U32: "eax", ast.U64: "rax"},
	"rbx": {ast.U8: "bl", ast.U16: "bx", ast.U32: "ebx", ast.U64: "rbx"},
	"rcx": {ast.U8: "cl", ast.U16: "cx", ast.U32: "ecx", ast.U64: "rcx"},
	"rdx": {ast.U8: "dl", ast.U16: "dx", ast.U32: "edx", ast.U64: "rdx"},
}

type registerInfo struct {
	register ir.Register
	declared diagnostic.Position
}

func Analyze(file, source string, program *ast.Program) (*ir.Program, error) {
	registers := make(map[string]registerInfo)
	labels := make(map[string]diagnostic.Position)
	result := &ir.Program{}

	for _, statement := range program.Statements {
		switch node := statement.(type) {
		case ast.RegisterDecl:
			if previous, exists := registers[node.Name]; exists {
				return nil, diagnostic.New(file, source, node.Pos, fmt.Sprintf("duplicate register '%s' (first declared at %d:%d)", node.Name, previous.declared.Line, previous.declared.Column))
			}
			if node.Pin != "" {
				return nil, diagnostic.New(file, source, node.Pos, "register pinning is not supported in v0.1")
			}
			if node.Type.Bits() == 0 {
				return nil, diagnostic.New(file, source, node.Pos, fmt.Sprintf("invalid type %q", node.Type))
			}
			if len(registers) >= len(physicalRegisters) {
				return nil, diagnostic.New(file, source, node.Pos, "v0.1 supports up to 4 virtual registers")
			}
			physical := physicalRegisters[len(registers)]
			register := ir.Register{Name: node.Name, Type: node.Type, Physical: physical, Pos: node.Pos}
			registers[node.Name] = registerInfo{register: register, declared: node.Pos}
			result.Registers = append(result.Registers, register)
		case ast.Label:
			if _, exists := labels[node.Name]; exists {
				return nil, diagnostic.New(file, source, node.Pos, fmt.Sprintf("duplicate label '%s'", node.Name))
			}
			labels[node.Name] = node.Pos
		}
	}

	lookupRegister := func(name string, pos diagnostic.Position) (ir.Register, error) {
		info, exists := registers[name]
		if !exists {
			return ir.Register{}, diagnostic.New(file, source, pos, fmt.Sprintf("undefined register '%s'", name))
		}
		if info.declared.Offset > pos.Offset {
			return ir.Register{}, diagnostic.New(file, source, pos, fmt.Sprintf("register '%s' is used before its declaration", name))
		}
		return info.register, nil
	}
	operand := func(value ast.Operand) (ir.Operand, error) {
		if value.IsRegister() {
			register, err := lookupRegister(value.Register, value.Pos)
			if err != nil {
				return ir.Operand{}, err
			}
			return ir.Operand{Register: registerAlias(register.Physical, register.Type), Type: register.Type}, nil
		}
		if value.Immediate != nil {
			return ir.Operand{Immediate: value.Immediate}, nil
		}
		return ir.Operand{}, diagnostic.New(file, source, value.Pos, "missing operand")
	}
	checkImmediate := func(value ir.Operand, targetType ast.Type, pos diagnostic.Position) error {
		if value.Immediate != nil && !fits(*value.Immediate, targetType) {
			return diagnostic.New(file, source, pos, fmt.Sprintf("integer %d does not fit in %s", *value.Immediate, targetType))
		}
		return nil
	}
	checkLabel := func(name string, pos diagnostic.Position) error {
		if _, exists := labels[name]; !exists {
			return diagnostic.New(file, source, pos, fmt.Sprintf("undefined label '%s'", name))
		}
		return nil
	}

	for _, statement := range program.Statements {
		switch node := statement.(type) {
		case ast.RegisterDecl:
			if node.Initializer != nil {
				if !fits(*node.Initializer, node.Type) {
					return nil, diagnostic.New(file, source, node.Pos, fmt.Sprintf("integer %d does not fit in %s", *node.Initializer, node.Type))
				}
				register := registers[node.Name].register
				value := *node.Initializer
				result.Instructions = append(result.Instructions, ir.Instruction{
					Op: ir.OpMove, Left: ir.Operand{Register: registerAlias(register.Physical, register.Type), Type: register.Type},
					Right: ir.Operand{Immediate: &value}, Pos: node.Pos,
				})
			}
		case ast.Label:
			result.Instructions = append(result.Instructions, ir.Instruction{Op: ir.OpLabel, Label: node.Name, Pos: node.Pos})
		case ast.Instruction:
			lowered := ir.Instruction{Pos: node.Pos}
			switch node.Op {
			case ast.Move, ast.Add, ast.Sub:
				if !node.Left.IsRegister() {
					return nil, diagnostic.New(file, source, node.Left.Pos, fmt.Sprintf("destination of '%s' must be a register", node.Op))
				}
				destination, err := lookupRegister(node.Left.Register, node.Left.Pos)
				if err != nil {
					return nil, err
				}
				left := ir.Operand{Register: registerAlias(destination.Physical, destination.Type), Type: destination.Type}
				right, err := operand(node.Right)
				if err != nil {
					return nil, err
				}
				if right.IsRegister() && right.Type != destination.Type {
					return nil, diagnostic.New(file, source, node.Right.Pos, fmt.Sprintf("register types do not match: %s and %s", destination.Type, right.Type))
				}
				if err := checkImmediate(right, destination.Type, node.Right.Pos); err != nil {
					return nil, err
				}
				lowered.Left, lowered.Right = left, right
				switch node.Op {
				case ast.Move:
					lowered.Op = ir.OpMove
				case ast.Add:
					lowered.Op = ir.OpAdd
				case ast.Sub:
					lowered.Op = ir.OpSub
				}
			case ast.Compare:
				if !node.Left.IsRegister() {
					return nil, diagnostic.New(file, source, node.Left.Pos, "left operand of 'compare' must be a register")
				}
				left, err := operand(node.Left)
				if err != nil {
					return nil, err
				}
				right, err := operand(node.Right)
				if err != nil {
					return nil, err
				}
				if right.IsRegister() && right.Type != left.Type {
					return nil, diagnostic.New(file, source, node.Right.Pos, fmt.Sprintf("register types do not match: %s and %s", left.Type, right.Type))
				}
				if err := checkImmediate(right, left.Type, node.Right.Pos); err != nil {
					return nil, err
				}
				lowered.Op, lowered.Left, lowered.Right = ir.OpCompare, left, right
			case ast.Jump:
				if err := checkLabel(node.Target, node.Pos); err != nil {
					return nil, err
				}
				lowered.Op, lowered.Label = ir.OpJump, node.Target
			case ast.JumpIf:
				if _, valid := jumpConditions[node.Condition]; !valid {
					return nil, diagnostic.New(file, source, node.Pos, fmt.Sprintf("unknown jump condition %q", node.Condition))
				}
				if err := checkLabel(node.Target, node.Pos); err != nil {
					return nil, err
				}
				lowered.Op, lowered.Condition, lowered.Label = ir.OpJumpIf, node.Condition, node.Target
			case ast.Halt:
				lowered.Op = ir.OpHalt
			}
			result.Instructions = append(result.Instructions, lowered)
		}
	}
	return result, nil
}

var jumpConditions = map[string]string{
	"equal": "je", "not_equal": "jne", "zero": "jz", "not_zero": "jnz",
	"less": "jb", "less_equal": "jbe", "greater": "ja", "greater_equal": "jae",
}

func registerAlias(register string, registerType ast.Type) string {
	return registerAliases[register][registerType]
}

func fits(value uint64, targetType ast.Type) bool {
	if targetType == ast.U64 {
		return true
	}
	bits := targetType.Bits()
	if bits == 0 {
		return false
	}
	return value <= (uint64(1)<<bits)-1
}
