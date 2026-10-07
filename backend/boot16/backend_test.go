package boot16

import (
	"strings"
	"testing"

	"inox/compiler/ast"
)

func TestGenerateBootstrapsBIOSAndLowersBIOSPrint(t *testing.T) {
	program := &ast.Program{Statements: []ast.Statement{
		ast.BIOSPrint{Text: "Hi"},
		ast.AssemblyBlock{Source: "    cli\n    hlt\n"},
	}}
	assembly := Generate(program)
	for _, want := range []string{
		"bits 16",
		"org 0x7c00",
		"mov sp, 0x7c00",
		"cld",
		"int 0x10",
		"__inox_text_0: db 0x48, 0x69, 0",
		"times 510 - ($ - $$) db 0",
		"dw 0xaa55",
	} {
		if !strings.Contains(assembly, want) {
			t.Errorf("generated assembly missing %q:\n%s", want, assembly)
		}
	}
}
