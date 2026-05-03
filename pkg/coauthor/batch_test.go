package coauthor

import "testing"

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
		got := escapeSed(tt.input)
		if got != tt.expected {
			t.Errorf("escapeSed(%q) = %q want %q", tt.input, got, tt.expected)
		}
	}
}

func TestEscapeSedEmail(t *testing.T) {
	input := "user@github.com"
	expected := `user@github\.com`
	got := escapeSed(input)
	if got != expected {
		t.Errorf("escapeSed(%q) = %q want %q", input, got, expected)
	}
}
