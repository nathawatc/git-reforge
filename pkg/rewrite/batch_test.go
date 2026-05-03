package rewrite

import "testing"

func TestEscapeShell(t *testing.T) {
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
		got := EscapeShell(tt.input)
		if got != tt.expected {
			t.Errorf("EscapeShell(%q) = %q want %q", tt.input, got, tt.expected)
		}
	}
}

func TestEscapeShellSingleQuote(t *testing.T) {
	// Single quote: end quote, add literal ', start new quote
	input := "with'single"
	expected := `with'\''single`
	got := EscapeShell(input)
	if got != expected {
		t.Errorf("EscapeShell(%q) = %q want %q", input, got, expected)
	}
}
