package lexer

import (
	"strings"
	"testing"
)

func TestLexRegistersInstructionsLabelsAndComments(t *testing.T) {
	source := "register count: u64 = 5\nsub count, 1 // decrement\nfinished:\nhalt\n"
	tokens, err := Lex("sample.ix", source)
	if err != nil {
		t.Fatal(err)
	}

	want := []struct {
		kind   TokenKind
		lexeme string
	}{
		{TokenIdentifier, "register"}, {TokenIdentifier, "count"}, {TokenColon, ":"},
		{TokenIdentifier, "u64"}, {TokenEqual, "="}, {TokenNumber, "5"},
		{TokenNewline, "\n"}, {TokenIdentifier, "sub"}, {TokenIdentifier, "count"},
		{TokenComma, ","}, {TokenNumber, "1"}, {TokenNewline, "\n"},
		{TokenIdentifier, "finished"}, {TokenColon, ":"}, {TokenNewline, "\n"},
		{TokenIdentifier, "halt"}, {TokenNewline, "\n"}, {TokenEOF, ""},
	}
	if len(tokens) != len(want) {
		t.Fatalf("got %d tokens, want %d", len(tokens), len(want))
	}
	for i, expected := range want {
		if tokens[i].Kind != expected.kind || tokens[i].Lexeme != expected.lexeme {
			t.Errorf("token %d = (%v, %q), want (%v, %q)", i, tokens[i].Kind, tokens[i].Lexeme, expected.kind, expected.lexeme)
		}
	}
}

func TestLexReportsInvalidCharacterLocation(t *testing.T) {
	_, err := Lex("bad.ix", "halt\n$")
	if err == nil || err.Error() != "error: unexpected character '$'\n --> bad.ix:2:1\n\n2 | $\n  | ^" {
		t.Fatalf("unexpected diagnostic: %v", err)
	}
}

func TestLexSupportsHashCommentsCRLFAndHexLiterals(t *testing.T) {
	tokens, err := Lex("comments.ix", "# header\r\nregister a: u16 = 0xff\r\n")
	if err != nil {
		t.Fatal(err)
	}
	var number Token
	for _, token := range tokens {
		if token.Kind == TokenNumber {
			number = token
		}
	}
	if number.Lexeme != "0xff" || number.Pos.Line != 2 || number.Pos.Column != 19 {
		t.Fatalf("number token = %#v", number)
	}
}

func TestLexRejectsNonASCIIIdentifierCharacters(t *testing.T) {
	_, err := Lex("unicode.ix", "register contador_β: u8\n")
	if err == nil || !strings.Contains(err.Error(), "unexpected character 'β'") {
		t.Fatalf("unexpected diagnostic: %v", err)
	}
}

func TestLexPreservesOpaqueAssemblyBlock(t *testing.T) {
	source := "target boot16\nasm {\n    mov ax, 0x7c00\n    jmp $ ; } in comment\n    db '}'\n}\nstage2 {\n    mov ax, 0x1234\n}\n"
	tokens, err := Lex("boot.ix", source)
	if err != nil {
		t.Fatal(err)
	}
	var block Token
	for _, token := range tokens {
		if token.Kind == TokenAssemblyBlock {
			block = token
			break
		}
	}
	if block.Kind != TokenAssemblyBlock {
		t.Fatal("assembly block token not found")
	}
	want := "\n    mov ax, 0x7c00\n    jmp $ ; } in comment\n    db '}'\n"
	if got := block.Lexeme; got != want {
		t.Fatalf("assembly block = %q, want %q", got, want)
	}
	for _, token := range tokens {
		if token.Kind == TokenStage2Block && strings.Contains(token.Lexeme, "mov ax, 0x1234") {
			return
		}
	}
	t.Fatal("stage2 block token not found")
}

func TestLexSupportsQuotedStringsForInstructions(t *testing.T) {
	tokens, err := Lex("print.ix", "bios_print \"Inox\\nboot\"\n")
	if err != nil {
		t.Fatal(err)
	}
	if got := tokens[1].Kind; got != TokenString {
		t.Fatalf("string token kind = %v, want string", got)
	}
}

func TestFormatListsTokensAndEndOfFile(t *testing.T) {
	tokens, err := Lex("format.ix", "halt\n")
	if err != nil {
		t.Fatal(err)
	}
	want := "1:1 identifier \"halt\"\n1:5 newline    \"\\n\"\n2:1 end of file\n"
	if got := Format(tokens); got != want {
		t.Fatalf("Format() = %q, want %q", got, want)
	}
}
