package rewrite

import (
	"testing"

	"github-to-codeberg/pkg/shell"
)

func TestEscapeShellFromShell(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"simple", "simple"},
		{`with\ backslash`, `with\\ backslash`},
		{`with"quote`, `with\"quote`},
		{"with$dollar", `with\$dollar`},
	}

	for _, tt := range tests {
		got := shell.EscapeShell(tt.input)
		if got != tt.expected {
			t.Errorf("shell.EscapeShell(%q) = %q want %q", tt.input, got, tt.expected)
		}
	}
}

func TestEscapeShellSingleQuoteFromShell(t *testing.T) {
	input := "with'single"
	expected := `with'\''single`
	got := shell.EscapeShell(input)
	if got != expected {
		t.Errorf("shell.EscapeShell(%q) = %q want %q", input, got, expected)
	}
}
