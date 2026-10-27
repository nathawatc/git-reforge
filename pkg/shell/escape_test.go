package shell

import (
	"os/exec"
	"strings"
	"testing"
)

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

func TestRunCommandCapturesOutputOnError(t *testing.T) {
	cmd := exec.Command("sh", "-c", "echo 'something went wrong' >&2; exit 42")
	err := RunCommand(cmd, "test command")
	if err == nil {
		t.Fatal("RunCommand() error = nil want error")
	}
	if !strings.Contains(err.Error(), "something went wrong") {
		t.Fatalf("RunCommand() error %q does not contain stderr output", err.Error())
	}
	if !strings.Contains(err.Error(), "test command failed") {
		t.Fatalf("RunCommand() error %q does not contain description", err.Error())
	}
}

func TestRunCommandSucceedsOnZeroExit(t *testing.T) {
	cmd := exec.Command("sh", "-c", "echo 'ok'; exit 0")
	if err := RunCommand(cmd, "successful command"); err != nil {
		t.Fatalf("RunCommand() error = %v want nil", err)
	}
}
