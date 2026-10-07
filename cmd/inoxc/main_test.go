package main

import (
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"inox/backend/boot16"
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

func TestBuildAssemblesBoot16ImageWithBIOSSignature(t *testing.T) {
	if _, err := exec.LookPath("nasm"); err != nil {
		t.Skip("nasm is not installed")
	}
	directory := t.TempDir()
	source := filepath.Join(directory, "boot.ix")
	include := filepath.Join(directory, "common.inc")
	output := filepath.Join(directory, "boot.img")
	if err := os.WriteFile(include, []byte("nop\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	program := "target boot16\nbios_print \"Inox\"\nasm {\n%include \"common.inc\"\n}\nstage2 {\nmov ax, 0x1234\n}\n"
	if err := os.WriteFile(source, []byte(program), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := run([]string{"build", source, "-o", output}); err != nil {
		t.Fatal(err)
	}
	image, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	if len(image) != boot16.FloppyImageSize {
		t.Fatalf("boot image is %d bytes, want a 1.44 MiB floppy image", len(image))
	}
	if image[510] != 0x55 || image[511] != 0xaa {
		t.Fatalf("boot signature = %02x %02x, want 55 aa", image[510], image[511])
	}
	if image[512] != 0xb8 || image[513] != 0x34 || image[514] != 0x12 {
		t.Fatalf("stage2 bytes = % x, want mov ax, 0x1234 at sector 2", image[512:515])
	}
}

func TestRunRequiresQEMUForBoot16Target(t *testing.T) {
	directory := t.TempDir()
	source := filepath.Join(directory, "boot.ix")
	if err := os.WriteFile(source, []byte("target boot16\nbios_print \"Inox\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := run([]string{"run", source}); err == nil || !strings.Contains(err.Error(), "needs QEMU") {
		t.Fatalf("run boot16 error = %v, want a QEMU requirement", err)
	}
}

func TestBoot16LoadsStage2AcrossCHSTrackBoundary(t *testing.T) {
	if _, err := exec.LookPath("nasm"); err != nil {
		t.Skip("nasm is not installed")
	}
	qemu, err := findQEMUSystem()
	if err != nil {
		t.Skip(err)
	}
	directory := t.TempDir()
	source := filepath.Join(directory, "track.ix")
	image := filepath.Join(directory, "track.img")
	program := "target boot16\nstage2 {\n    jmp stage2_entry\n    times 19 * 512 db 0\nstage2_entry:\n    mov dx, 0xf4\n    mov al, 0x10\n    out dx, al\n}\n"
	if err := os.WriteFile(source, []byte(program), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := run([]string{"build", source, "-o", image}); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, qemu, "-display", "none", "-machine", "pc", "-m", "16M", "-drive", "file="+image+",format=raw,if=floppy", "-boot", "order=a", "-no-reboot", "-monitor", "none", "-serial", "none", "-device", "isa-debug-exit,iobase=0xf4,iosize=0x04")
	err = command.Run()
	var exitError *exec.ExitError
	if !errors.As(err, &exitError) || exitError.ExitCode() != 33 {
		t.Fatalf("QEMU exit = %v, want isa-debug-exit code 33", err)
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
