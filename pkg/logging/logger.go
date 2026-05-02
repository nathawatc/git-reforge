package logging

import (
	"fmt"
	"log/slog"
	"os"
	"strings"
)

const (
	Red    = "\033[1;31m"
	Green  = "\033[1;32m"
	Yellow = "\033[1;33m"
	Blue   = "\033[0;34m"
	Cyan   = "\033[0;36m"
	Bold   = "\033[1m"
	NC     = "\033[0m"
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
	fmt.Printf("%s[%s]%s %s\n", color, level, NC, msg)
}

func (l *Logger) Info(msg string) {
	l.log(Blue, "INFO", msg)
}

func (l *Logger) Success(msg string) {
	l.log(Green, "SUCCESS", msg)
}

func (l *Logger) Warning(msg string) {
	l.log(Yellow, "WARNING", msg)
}

func (l *Logger) Error(msg string) {
	l.log(Red, "ERROR", msg)
}

func (l *Logger) Header(msg string) {
	fmt.Println()
	fmt.Println(strings.Repeat("═", 64))
	fmt.Printf("%s%s%s %s%s%s\n", Bold, Cyan, "  ", msg, NC)
	fmt.Println(strings.Repeat("═", 64))
	fmt.Println()
}

func (l *Logger) Printf(format string, args ...interface{}) {
	fmt.Printf(format, args...)
}
