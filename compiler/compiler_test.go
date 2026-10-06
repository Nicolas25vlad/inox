package compiler

import (
	"strings"
	"testing"
)

func TestCompileRunsTheFullPipeline(t *testing.T) {
	result, err := Compile("hello.ix", "register value: u64 = 42\nhalt\n")
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Tokens) == 0 || result.AST == nil || result.IR == nil || !strings.Contains(result.Assembly, "mov rax, 42") {
		t.Fatalf("incomplete compiler result: %#v", result)
	}
}

func TestCompileReturnsLexerParserAndSemanticErrors(t *testing.T) {
	for _, source := range []string{
		"halt\n$\n",
		"register : u64\n",
		"register a: u8\nregister a: u8\n",
	} {
		if _, err := Compile("bad.ix", source); err == nil || !strings.Contains(err.Error(), "error:") {
			t.Errorf("Compile(%q) error = %v, want a source error", source, err)
		}
	}
}
