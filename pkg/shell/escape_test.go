package shell

import (
	"os/exec"
	"strings"
	"testing"
)

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
