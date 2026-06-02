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

func TestEscapeDoubleQuoted(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"simple", "simple"},
		{`with\ backslash`, `with\\ backslash`},
		{`with"quote`, `with\"quote`},
		{"with$dollar", `with\$dollar`},
		{"with'single", "with'single"},
	}

	for _, tt := range tests {
		got := EscapeDoubleQuoted(tt.input)
		if got != tt.want {
			t.Errorf("EscapeDoubleQuoted(%q) = %q want %q", tt.input, got, tt.want)
		}
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

func TestEscapeSedReplacement(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"user.name@example.org", "user.name@example.org"},
		{"A&B", `A\&B`},
		{"with/slash", `with\/slash`},
		{`with\backslash`, `with\\backslash`},
	}

	for _, tt := range tests {
		got := EscapeSedReplacement(tt.input)
		if got != tt.want {
			t.Errorf("EscapeSedReplacement(%q) = %q want %q", tt.input, got, tt.want)
		}
	}
}

func TestQuoteSingle(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"simple", "'simple'"},
		{"with space", "'with space'"},
		{"with'single", "'with'\\''single'"},
	}

	for _, tt := range tests {
		got := QuoteSingle(tt.input)
		if got != tt.want {
			t.Errorf("QuoteSingle(%q) = %q want %q", tt.input, got, tt.want)
		}
	}
}
