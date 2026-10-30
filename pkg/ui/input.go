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
	if tty, err := os.OpenFile("/dev/tty", os.O_RDWR, 0); err == nil {
		return &PromptSession{reader: bufio.NewReader(tty), output: tty, file: tty}
	}
	return &PromptSession{reader: bufio.NewReader(os.Stdin), output: os.Stdout}
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
