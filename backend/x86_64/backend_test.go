package x86_64

import (
	"strings"
	"testing"

	"inox/compiler/parser"
	"inox/compiler/semantic"
)

func TestGenerateEmitsNASMProgramAndLinuxExit(t *testing.T) {
	source := "register a: u64 = 5\nregister b: u64 = 10\nadd a, b\ncompare a, b\njump_if equal, finished\njump finished\nfinished:\nhalt\n"
	program, err := parser.Parse("basic.ix", source)
	if err != nil {
		t.Fatal(err)
	}
	irProgram, err := semantic.Analyze("basic.ix", source, program)
	if err != nil {
		t.Fatal(err)
	}
	assembly := Generate(irProgram)
	for _, want := range []string{
		"bits 64", "global _start", "section .text", "_start:",
		"mov rax, 5", "mov rbx, 10", "add rax, rbx", "cmp rax, rbx",
		"je inox_l_finished", "jmp inox_l_finished", "inox_l_finished:", "mov eax, 60", "syscall",
	} {
		if !strings.Contains(assembly, want) {
			t.Errorf("assembly missing %q:\n%s", want, assembly)
		}
	}
	if count := strings.Count(assembly, "syscall"); count != 1 {
		t.Fatalf("got %d exit syscalls for a final halt, want 1", count)
	}
}

func TestGenerateManglesAssemblerReservedLabels(t *testing.T) {
	source := "jump rax\nrax:\nhalt\n"
	program, err := parser.Parse("reserved.ix", source)
	if err != nil {
		t.Fatal(err)
	}
	irProgram, err := semantic.Analyze("reserved.ix", source, program)
	if err != nil {
		t.Fatal(err)
	}
	assembly := Generate(irProgram)
	if !strings.Contains(assembly, "jmp inox_l_rax") || !strings.Contains(assembly, "inox_l_rax:") {
		t.Fatalf("label was not safely mangled:\n%s", assembly)
	}
}

func TestGenerateUsesPinnedRegisterAliases(t *testing.T) {
	source := "register byte: u8 @r8\nsub byte, 1\nhalt\n"
	program, err := parser.Parse("pin.ix", source)
	if err != nil {
		t.Fatal(err)
	}
	irProgram, err := semantic.Analyze("pin.ix", source, program)
	if err != nil {
		t.Fatal(err)
	}
	if assembly := Generate(irProgram); !strings.Contains(assembly, "sub r8b, 1") {
		t.Fatalf("pinned byte register alias missing:\n%s", assembly)
	}
}

func TestGenerateUsesScratchForLarge64BitArithmeticImmediate(t *testing.T) {
	source := "register value: u64\nadd value, 18446744073709551615\nhalt\n"
	program, err := parser.Parse("large.ix", source)
	if err != nil {
		t.Fatal(err)
	}
	irProgram, err := semantic.Analyze("large.ix", source, program)
	if err != nil {
		t.Fatal(err)
	}
	assembly := Generate(irProgram)
	if !strings.Contains(assembly, "mov r11, 18446744073709551615\n    add rax, r11") {
		t.Fatalf("large immediate was not lowered through scratch register:\n%s", assembly)
	}
}
