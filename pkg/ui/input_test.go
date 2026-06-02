package ui

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"
)

func TestPromptSessionReadLineTrimsInputAndWritesPrompt(t *testing.T) {
	var output bytes.Buffer
	prompts := newPromptSession(strings.NewReader("  yes  \n"), &output, nil)

	got, err := prompts.ReadLine("Proceed? ")
	if err != nil {
		t.Fatalf("ReadLine() error = %v", err)
	}

	if got != "yes" {
		t.Fatalf("ReadLine() = %q want yes", got)
	}
	if output.String() != "Proceed? " {
		t.Fatalf("prompt output = %q want %q", output.String(), "Proceed? ")
	}
}

func TestPromptSessionReadLineReturnsErrorOnEOF(t *testing.T) {
	var output bytes.Buffer
	prompts := newPromptSession(strings.NewReader(""), &output, nil)

	got, err := prompts.ReadLine("Choice: ")
	if err == nil {
		t.Fatalf("ReadLine() error = nil want EOF")
	}
	if !errors.Is(err, io.EOF) {
		t.Fatalf("ReadLine() error = %v want EOF", err)
	}
	if got != "" {
		t.Fatalf("ReadLine() = %q want empty", got)
	}
}
