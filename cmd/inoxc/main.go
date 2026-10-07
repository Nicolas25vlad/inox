package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"inox/backend/boot16"
	"inox/compiler"
	"inox/compiler/ast"
	"inox/compiler/lexer"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		var exitError *exec.ExitError
		if errors.As(err, &exitError) {
			os.Exit(exitError.ExitCode())
		}
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		return usageError()
	}
	switch args[0] {
	case "build":
		return build(args[1:])
	case "inspect":
		return inspect(args[1:])
	case "run":
		return runProgram(args[1:])
	default:
		return usageError()
	}
}

func runProgram(args []string) error {
	flags := flag.NewFlagSet("run", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	vm := flags.Bool("vm", false, "run through QEMU (user-mode for Linux, system emulator for boot16)")
	if err := flags.Parse(flagsBeforePositionals(args, nil)); err != nil {
		return err
	}
	if flags.NArg() != 1 {
		return fmt.Errorf("usage: inoxc run [--vm] file.ix")
	}
	input := flags.Arg(0)
	result, err := compileFile(input)
	if err != nil {
		return err
	}
	if result.Target == ast.TargetBoot16 {
		if !*vm {
			return fmt.Errorf("the boot16 target needs QEMU; run it with --vm")
		}
		return runBoot16(result, input)
	}
	var runner string
	if *vm {
		runner, err = findQEMUUser()
		if err != nil {
			return err
		}
	}
	nasm, err := exec.LookPath("nasm")
	if err != nil {
		return fmt.Errorf("nasm is required for run: %w", err)
	}
	linker, err := exec.LookPath("ld")
	if err != nil {
		return fmt.Errorf("ld is required for run: %w", err)
	}
	tempDir, err := os.MkdirTemp("", "inox-run-")
	if err != nil {
		return fmt.Errorf("create temporary build directory: %w", err)
	}
	defer os.RemoveAll(tempDir)

	assembly := filepath.Join(tempDir, "program.asm")
	object := filepath.Join(tempDir, "program.o")
	executable := filepath.Join(tempDir, "program")
	if err := os.WriteFile(assembly, []byte(result.Assembly), 0o600); err != nil {
		return fmt.Errorf("write temporary assembly: %w", err)
	}
	if err := runTool(nasm, "-felf64", assembly, "-o", object); err != nil {
		return fmt.Errorf("assemble with NASM: %w", err)
	}
	if err := runTool(linker, "-o", executable, object); err != nil {
		return fmt.Errorf("link ELF: %w", err)
	}
	if runner == "" {
		return runTool(executable)
	}
	if err := runTool(runner, executable); err != nil {
		return fmt.Errorf("run with QEMU: %w", err)
	}
	return nil
}

func findQEMUUser() (string, error) {
	for _, name := range []string{"qemu-x86_64", "qemu-x86_64-static"} {
		if path, err := exec.LookPath(name); err == nil {
			return path, nil
		}
	}
	return "", fmt.Errorf("qemu-x86_64 is required for --vm (qemu-x86_64-static is also supported)")
}

func findQEMUSystem() (string, error) {
	for _, name := range []string{"qemu-system-x86_64", "qemu-system-i386"} {
		if path, err := exec.LookPath(name); err == nil {
			return path, nil
		}
	}
	return "", fmt.Errorf("qemu-system-x86_64 is required for the boot16 target")
}

func runBoot16(result *compiler.Result, sourcePath string) error {
	qemu, err := findQEMUSystem()
	if err != nil {
		return err
	}
	tempDir, err := os.MkdirTemp("", "inox-boot16-")
	if err != nil {
		return fmt.Errorf("create temporary boot image directory: %w", err)
	}
	defer os.RemoveAll(tempDir)
	image := filepath.Join(tempDir, "boot.img")
	if err := assembleBootImage(result.Assembly, image, filepath.Dir(sourcePath)); err != nil {
		return err
	}
	return runTool(qemu, "-machine", "pc", "-m", "16M", "-drive", "file="+image+",format=raw,if=floppy", "-boot", "order=a")
}

func runTool(path string, args ...string) error {
	command := exec.Command(path, args...)
	command.Stdin = os.Stdin
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	return command.Run()
}

func build(args []string) error {
	flags := flag.NewFlagSet("build", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	output := flags.String("o", "", "output file (assembly for Linux, boot image for boot16)")
	if err := flags.Parse(flagsBeforePositionals(args, map[string]bool{"-o": true})); err != nil {
		return err
	}
	if flags.NArg() != 1 {
		return fmt.Errorf("usage: inoxc build [-o output.asm] file.ix")
	}
	input := flags.Arg(0)
	result, err := compileFile(input)
	if err != nil {
		return err
	}
	isBoot16 := result.Target == ast.TargetBoot16
	path := *output
	if path == "" {
		base := strings.TrimSuffix(filepath.Base(input), filepath.Ext(input))
		extension := ".asm"
		if isBoot16 {
			extension = ".img"
		}
		path = filepath.Join("build", base+extension)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create output directory: %w", err)
	}
	if isBoot16 {
		if err := assembleBootImage(result.Assembly, path, filepath.Dir(input)); err != nil {
			return err
		}
		fmt.Printf("generated boot image %s\n", path)
		return nil
	}
	if err := os.WriteFile(path, []byte(result.Assembly), 0o644); err != nil {
		return fmt.Errorf("write assembly: %w", err)
	}
	fmt.Printf("generated %s\n", path)
	return nil
}

func inspect(args []string) error {
	flags := flag.NewFlagSet("inspect", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	tokens := flags.Bool("tokens", false, "print lexer tokens")
	astOutput := flags.Bool("ast", false, "print parsed AST")
	irOutput := flags.Bool("ir", false, "print lowered IR")
	assembly := flags.Bool("asm", false, "print NASM assembly")
	if err := flags.Parse(flagsBeforePositionals(args, nil)); err != nil {
		return err
	}
	if flags.NArg() != 1 {
		return fmt.Errorf("usage: inoxc inspect [--tokens] [--ast] [--ir] [--asm] file.ix")
	}
	result, err := compileFile(flags.Arg(0))
	if err != nil {
		return err
	}
	selected := *tokens || *astOutput || *irOutput || *assembly
	if *tokens {
		fmt.Print(lexer.Format(result.Tokens))
	}
	if *astOutput {
		fmt.Print(ast.Format(result.AST))
	}
	if *irOutput {
		if result.IR == nil {
			fmt.Println("the boot16 target uses direct assembly and has no virtual-register IR")
		} else {
			fmt.Print(result.IR.String())
		}
	}
	if *assembly || !selected {
		fmt.Print(result.Assembly)
	}
	return nil
}

func compileFile(path string) (*compiler.Result, error) {
	source, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	return compiler.Compile(path, string(source))
}

func assembleBootImage(assembly, output, includeDirectory string) error {
	nasm, err := exec.LookPath("nasm")
	if err != nil {
		return fmt.Errorf("nasm is required to build boot16 images: %w", err)
	}
	tempDir, err := os.MkdirTemp("", "inox-nasm-")
	if err != nil {
		return fmt.Errorf("create temporary assembly directory: %w", err)
	}
	defer os.RemoveAll(tempDir)
	assemblyPath := filepath.Join(tempDir, "boot.asm")
	if err := os.WriteFile(assemblyPath, []byte(assembly), 0o600); err != nil {
		return fmt.Errorf("write temporary boot assembly: %w", err)
	}
	includeDirectory, err = filepath.Abs(includeDirectory)
	if err != nil {
		return fmt.Errorf("resolve source directory for NASM includes: %w", err)
	}
	includePath := includeDirectory + string(os.PathSeparator)
	if err := runTool(nasm, "-fbin", "-I", includePath, assemblyPath, "-o", output); err != nil {
		return fmt.Errorf("assemble boot image with NASM: %w", err)
	}
	info, err := os.Stat(output)
	if err != nil {
		return fmt.Errorf("inspect generated boot image: %w", err)
	}
	if info.Size() != boot16.FloppyImageSize {
		return fmt.Errorf("boot image must be a 1.44 MiB floppy; got %d bytes", info.Size())
	}
	return nil
}

func flagsBeforePositionals(args []string, valueFlags map[string]bool) []string {
	options := make([]string, 0, len(args))
	positionals := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			positionals = append(positionals, args[i+1:]...)
			break
		}
		if !strings.HasPrefix(arg, "-") {
			positionals = append(positionals, arg)
			continue
		}
		options = append(options, arg)
		name, _, hasValue := strings.Cut(arg, "=")
		if valueFlags[name] && !hasValue && i+1 < len(args) {
			i++
			options = append(options, args[i])
		}
	}
	return append(options, positionals...)
}

func usageError() error { return fmt.Errorf("usage: inoxc <build|inspect|run> ...") }
