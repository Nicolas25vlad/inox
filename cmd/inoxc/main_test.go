package main

import (
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildAcceptsOutputFlagAfterSource(t *testing.T) {
	directory := t.TempDir()
	source := filepath.Join(directory, "program.ix")
	output := filepath.Join(directory, "program.asm")
	if err := os.WriteFile(source, []byte("register value: u64 = 42\nhalt\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := run([]string{"build", source, "-o", output}); err != nil {
		t.Fatal(err)
	}
	assembly, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(assembly), "mov rax, 42") {
		t.Fatalf("unexpected output assembly:\n%s", assembly)
	}
}

func TestInspectAcceptsModeFlagAfterSource(t *testing.T) {
	directory := t.TempDir()
	source := filepath.Join(directory, "program.ix")
	if err := os.WriteFile(source, []byte("halt\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	output := captureStdout(t, func() {
		if err := run([]string{"inspect", source, "--ir"}); err != nil {
			t.Error(err)
		}
	})
	if !strings.Contains(output, "halt\n") || strings.Contains(output, "global _start") {
		t.Fatalf("inspect --ir output = %q", output)
	}
}

func TestRunExecutesProgramNatively(t *testing.T) {
	if _, err := exec.LookPath("nasm"); err != nil {
		t.Skip("nasm is not installed")
	}
	if _, err := exec.LookPath("ld"); err != nil {
		t.Skip("ld is not installed")
	}
	assertRunSucceeds(t, false)
}

func TestRunExecutesProgramWithQEMUUserMode(t *testing.T) {
	if _, err := exec.LookPath("nasm"); err != nil {
		t.Skip("nasm is not installed")
	}
	if _, err := exec.LookPath("ld"); err != nil {
		t.Skip("ld is not installed")
	}
	if _, err := findQEMUUser(); err != nil {
		t.Skip(err)
	}
	assertRunSourceSucceeds(t, "register byte: u8 = 42 @r8\nsub byte, 1\nhalt\n", true)
}

func assertRunSucceeds(t *testing.T, vm bool) {
	assertRunSourceSucceeds(t, "register value: u64 = 42\nhalt\n", vm)
}

func assertRunSourceSucceeds(t *testing.T, program string, vm bool) {
	t.Helper()
	directory := t.TempDir()
	source := filepath.Join(directory, "program.ix")
	if err := os.WriteFile(source, []byte(program), 0o600); err != nil {
		t.Fatal(err)
	}
	args := []string{"run", source}
	if vm {
		args = append(args, "--vm")
	}
	if err := run(args); err != nil {
		t.Fatalf("run(vm=%v): %v", vm, err)
	}
}

func captureStdout(t *testing.T, action func()) string {
	t.Helper()
	read, write, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	previous := os.Stdout
	os.Stdout = write
	defer func() { os.Stdout = previous }()
	action()
	if err := write.Close(); err != nil {
		t.Fatal(err)
	}
	output, err := io.ReadAll(read)
	if err != nil {
		t.Fatal(err)
	}
	if err := read.Close(); err != nil {
		t.Fatal(err)
	}
	return string(output)
}
