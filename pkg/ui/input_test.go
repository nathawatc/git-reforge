package ui

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"
)

func TestPromptSessionReadLineTrimsInputAndWritesPrompt(t *testing.T) {
	var output bytes.Buffer
	prompts := &PromptSession{reader: bufio.NewReader(strings.NewReader("  yes  \n")), output: &output}

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
	prompts := &PromptSession{reader: bufio.NewReader(strings.NewReader("")), output: &output}

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
