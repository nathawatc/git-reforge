package shell

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
	input := "with'single"
	expected := `with'\''single`
	got := EscapeShell(input)
	if got != expected {
		t.Errorf("EscapeShell(%q) = %q want %q", input, got, expected)
	}
}

func TestEscapeSed(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"simple", "simple"},
		{"with/slash", "with\\/slash"},
		{"with[brackets]", "with\\[brackets\\]"},
		{"with.dot", "with\\.dot"},
	}

	for _, tt := range tests {
		got := EscapeSed(tt.input)
		if got != tt.expected {
			t.Errorf("EscapeSed(%q) = %q want %q", tt.input, got, tt.expected)
		}
	}
}

func TestEscapeSedEmail(t *testing.T) {
	input := "user@github.com"
	expected := `user@github\.com`
	got := EscapeSed(input)
	if got != expected {
		t.Errorf("EscapeSed(%q) = %q want %q", input, got, expected)
	}
}
