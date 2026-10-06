package diagnostic

import "testing"

func TestErrorRendersSourceLocationAndCaret(t *testing.T) {
	err := New("sample.ix", "halt\nregister : u64\n", Position{Line: 2, Column: 10}, "expected register name")
	want := "error: expected register name\n --> sample.ix:2:10\n\n2 | register : u64\n  |          ^"
	if got := err.Error(); got != want {
		t.Fatalf("diagnostic = %q, want %q", got, want)
	}
}

func TestErrorWithoutPositionOmitsSourceExcerpt(t *testing.T) {
	err := (&Error{Message: "usage: inoxc"}).Error()
	if err != "error: usage: inoxc" {
		t.Fatalf("diagnostic = %q", err)
	}
}
