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

func useColor() bool {
	return os.Getenv("NO_COLOR") == ""
}

func (l *Logger) Info(msg string) {
	if useColor() {
		fmt.Printf("%s[INFO]%s %s\n", ui.Blue, ui.NC, msg)
	} else {
		fmt.Printf("[INFO] %s\n", msg)
	}
}

func (l *Logger) Success(msg string) {
	if useColor() {
		fmt.Printf("%s[SUCCESS]%s %s\n", ui.Green, ui.NC, msg)
	} else {
		fmt.Printf("[SUCCESS] %s\n", msg)
	}
}

func (l *Logger) Warning(msg string) {
	if useColor() {
		fmt.Fprintf(os.Stderr, "%s[WARNING]%s %s\n", ui.Yellow, ui.NC, msg)
	} else {
		fmt.Fprintf(os.Stderr, "[WARNING] %s\n", msg)
	}
}

func (l *Logger) Error(msg string) {
	if useColor() {
		fmt.Fprintf(os.Stderr, "%s[ERROR]%s %s\n", ui.Red, ui.NC, msg)
	} else {
		fmt.Fprintf(os.Stderr, "[ERROR] %s\n", msg)
	}
}

func (l *Logger) Header(msg string) {
	fmt.Println()
	fmt.Println(strings.Repeat("═", 64))
	if useColor() {
		fmt.Printf("%s%s  %s%s\n", ui.Bold, ui.Cyan, msg, ui.NC)
	} else {
		fmt.Printf("  %s\n", msg)
	}
	fmt.Println(strings.Repeat("═", 64))
	fmt.Println()
}
