package ui

import (
	"bytes"
	"strings"
	"testing"
)

func TestPromptSessionReadLineTrimsInputAndWritesPrompt(t *testing.T) {
	var output bytes.Buffer
	prompts := newPromptSession(strings.NewReader("  yes  \n"), &output, nil)

	got := prompts.ReadLine("Proceed? ")

	if got != "yes" {
		t.Fatalf("ReadLine() = %q want yes", got)
	}
	if output.String() != "Proceed? " {
		t.Fatalf("prompt output = %q want %q", output.String(), "Proceed? ")
	}
}

func TestPromptSessionReadLineReturnsEmptyOnEOF(t *testing.T) {
	var output bytes.Buffer
	prompts := newPromptSession(strings.NewReader(""), &output, nil)

	got := prompts.ReadLine("Choice: ")

	if got != "" {
		t.Fatalf("ReadLine() = %q want empty", got)
	}
}
