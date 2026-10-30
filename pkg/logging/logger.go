package logging

import (
	"fmt"
	"os"
	"strings"

	"git-reforge/pkg/ui"
)

type Logger struct{}

func New() *Logger {
	return &Logger{}
}

func (l *Logger) Info(msg string) {
	fmt.Printf("%s[INFO]%s %s\n", ui.Blue, ui.NC, msg)
}

func (l *Logger) Success(msg string) {
	fmt.Printf("%s[SUCCESS]%s %s\n", ui.Green, ui.NC, msg)
}

func (l *Logger) Warning(msg string) {
	fmt.Fprintf(os.Stderr, "%s[WARNING]%s %s\n", ui.Yellow, ui.NC, msg)
}

func (l *Logger) Error(msg string) {
	fmt.Fprintf(os.Stderr, "%s[ERROR]%s %s\n", ui.Red, ui.NC, msg)
}

func (l *Logger) Header(msg string) {
	fmt.Println()
	fmt.Println(strings.Repeat("═", 64))
	fmt.Printf("%s%s  %s%s\n", ui.Bold, ui.Cyan, msg, ui.NC)
	fmt.Println(strings.Repeat("═", 64))
	fmt.Println()
}
