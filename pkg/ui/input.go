package ui

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
)

type PromptSession struct {
	reader *bufio.Reader
	output io.Writer
	file   *os.File
}

func NewPromptSession() *PromptSession {
	tty, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
	if err == nil {
		return newPromptSession(tty, tty, tty)
	}

	return newPromptSession(os.Stdin, os.Stdout, nil)
}

func newPromptSession(input io.Reader, output io.Writer, file *os.File) *PromptSession {
	return &PromptSession{
		reader: bufio.NewReader(input),
		output: output,
		file:   file,
	}
}

func (p *PromptSession) Close() error {
	if p.file == nil {
		return nil
	}
	if err := p.file.Close(); err != nil {
		return fmt.Errorf("failed to close prompt session: %w", err)
	}
	return nil
}

func (p *PromptSession) ReadLine(prompt string) (string, error) {
	if _, err := fmt.Fprint(p.output, prompt); err != nil {
		return "", fmt.Errorf("failed to write prompt: %w", err)
	}
	input, err := p.reader.ReadString('\n')
	if err != nil {
		return "", fmt.Errorf("failed to read input: %w", err)
	}
	return strings.TrimSpace(input), nil
}
