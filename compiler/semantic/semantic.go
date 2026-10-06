package semantic

import (
	"fmt"

	"inox/compiler/ast"
	"inox/compiler/diagnostic"
	"inox/compiler/ir"
)

var physicalRegisters = []string{
	"rax", "rbx", "rcx", "rdx", "rsi", "rdi", "rbp",
	"r8", "r9", "r10", "r12", "r13", "r14", "r15",
}

var registerAliases = map[string]map[ast.Type]string{
	"rax": {ast.U8: "al", ast.U16: "ax", ast.U32: "eax", ast.U64: "rax"},
	"rbx": {ast.U8: "bl", ast.U16: "bx", ast.U32: "ebx", ast.U64: "rbx"},
	"rcx": {ast.U8: "cl", ast.U16: "cx", ast.U32: "ecx", ast.U64: "rcx"},
	"rdx": {ast.U8: "dl", ast.U16: "dx", ast.U32: "edx", ast.U64: "rdx"},
	"rsi": {ast.U8: "sil", ast.U16: "si", ast.U32: "esi", ast.U64: "rsi"},
	"rdi": {ast.U8: "dil", ast.U16: "di", ast.U32: "edi", ast.U64: "rdi"},
	"rbp": {ast.U8: "bpl", ast.U16: "bp", ast.U32: "ebp", ast.U64: "rbp"},
	"r8":  {ast.U8: "r8b", ast.U16: "r8w", ast.U32: "r8d", ast.U64: "r8"},
	"r9":  {ast.U8: "r9b", ast.U16: "r9w", ast.U32: "r9d", ast.U64: "r9"},
	"r10": {ast.U8: "r10b", ast.U16: "r10w", ast.U32: "r10d", ast.U64: "r10"},
	"r12": {ast.U8: "r12b", ast.U16: "r12w", ast.U32: "r12d", ast.U64: "r12"},
	"r13": {ast.U8: "r13b", ast.U16: "r13w", ast.U32: "r13d", ast.U64: "r13"},
	"r14": {ast.U8: "r14b", ast.U16: "r14w", ast.U32: "r14d", ast.U64: "r14"},
	"r15": {ast.U8: "r15b", ast.U16: "r15w", ast.U32: "r15d", ast.U64: "r15"},
}

type registerInfo struct {
	register ir.Register
	declared diagnostic.Position
}

func Analyze(file, source string, program *ast.Program) (*ir.Program, error) {
	registers := make(map[string]registerInfo)
	declarations := make([]ast.RegisterDecl, 0)
	declaredNames := make(map[string]diagnostic.Position)
	pinnedOwners := make(map[string]string)
	labels := make(map[string]diagnostic.Position)
	result := &ir.Program{}

	for _, statement := range program.Statements {
		switch node := statement.(type) {
		case ast.RegisterDecl:
			if previous, exists := declaredNames[node.Name]; exists {
				return nil, diagnostic.New(file, source, node.Pos, fmt.Sprintf("duplicate register '%s' (first declared at %d:%d)", node.Name, previous.Line, previous.Column))
			}
			declaredNames[node.Name] = node.Pos
			if node.Type.Bits() == 0 {
				return nil, diagnostic.New(file, source, node.Pos, fmt.Sprintf("invalid type %q", node.Type))
			}
			if node.Pin != "" {
				pinPos := node.PinPos
				if pinPos.Line == 0 {
					pinPos = node.Pos
				}
				if node.Pin == "rsp" {
					return nil, diagnostic.New(file, source, pinPos, "'rsp' is reserved for the stack pointer")
				}
				if node.Pin == "r11" {
					return nil, diagnostic.New(file, source, pinPos, "'r11' is reserved by the x86-64 backend")
				}
				if _, valid := registerAliases[node.Pin]; !valid {
					return nil, diagnostic.New(file, source, pinPos, fmt.Sprintf("unknown physical register '%s'", node.Pin))
				}
				if previous, exists := pinnedOwners[node.Pin]; exists {
					return nil, diagnostic.New(file, source, pinPos, fmt.Sprintf("physical register '%s' is pinned more than once (also used by '%s')", node.Pin, previous))
				}
				pinnedOwners[node.Pin] = node.Name
			}
			declarations = append(declarations, node)
		case ast.Label:
			if _, exists := labels[node.Name]; exists {
				return nil, diagnostic.New(file, source, node.Pos, fmt.Sprintf("duplicate label '%s'", node.Name))
			}
			labels[node.Name] = node.Pos
		}
	}
	usedPhysical := make(map[string]bool, len(pinnedOwners))
	for physical := range pinnedOwners {
		usedPhysical[physical] = true
	}
	for _, declaration := range declarations {
		physical := declaration.Pin
		if physical == "" {
			for _, candidate := range physicalRegisters {
				if !usedPhysical[candidate] {
					physical = candidate
					usedPhysical[physical] = true
					break
				}
			}
		}
		if physical == "" {
			return nil, diagnostic.New(file, source, declaration.Pos, fmt.Sprintf("no physical registers available for virtual register '%s'", declaration.Name))
		}
		register := ir.Register{Name: declaration.Name, Type: declaration.Type, Physical: physical, Pos: declaration.Pos}
		registers[declaration.Name] = registerInfo{register: register, declared: declaration.Pos}
		result.Registers = append(result.Registers, register)
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
