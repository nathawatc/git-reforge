package logging

import (
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github-to-codeberg/pkg/ui"
)

type Logger struct {
	*slog.Logger
}

func New() *Logger {
	opts := &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}
	handler := slog.NewJSONHandler(os.Stdout, opts)
	return &Logger{slog.New(handler)}
}

func (l *Logger) log(color string, level string, msg string) {
	fmt.Printf("%s[%s]%s %s\n", color, level, ui.NC, msg)
}

func (l *Logger) Info(msg string) {
	l.log(ui.Blue, "INFO", msg)
}

func (l *Logger) Success(msg string) {
	l.log(ui.Green, "SUCCESS", msg)
}

func (l *Logger) Warning(msg string) {
	l.log(ui.Yellow, "WARNING", msg)
}

func (l *Logger) Error(msg string) {
	l.log(ui.Red, "ERROR", msg)
}

func (l *Logger) Header(msg string) {
	fmt.Println()
	fmt.Println(strings.Repeat("═", 64))
	fmt.Printf("%s%s%s %s%s\n", ui.Bold, ui.Cyan, "  ", msg, ui.NC)
	fmt.Println(strings.Repeat("═", 64))
	fmt.Println()
}

func (l *Logger) Printf(format string, args ...interface{}) {
	fmt.Printf(format, args...)
}
