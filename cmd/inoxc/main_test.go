package main

import (
	"io"
	"os"
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
