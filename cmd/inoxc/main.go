package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"inox/compiler"
	"inox/compiler/ast"
	"inox/compiler/lexer"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
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
	default:
		return usageError()
	}
}

func build(args []string) error {
	flags := flag.NewFlagSet("build", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	output := flags.String("o", "", "output NASM file")
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
	path := *output
	if path == "" {
		base := strings.TrimSuffix(filepath.Base(input), filepath.Ext(input))
		path = filepath.Join("build", base+".asm")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create output directory: %w", err)
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
		fmt.Print(result.IR.String())
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

func usageError() error { return fmt.Errorf("usage: inoxc <build|inspect> ...") }
