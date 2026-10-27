package shell

import (
	"fmt"
	"os/exec"
	"strings"
)

func RunCommand(cmd *exec.Cmd, description string) error {
	output, err := cmd.CombinedOutput()
	if err != nil {
		if len(output) > 0 {
			return fmt.Errorf("%s failed: %w\n%s", description, err, output)
		}
		return fmt.Errorf("%s failed: %w", description, err)
	}
	return nil
}

func HasFilterRepo() bool {
	_, err := exec.LookPath("git-filter-repo")
	return err == nil
}

func EscapeDoubleQuoted(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "\"", "\\\"")
	s = strings.ReplaceAll(s, "`", "\\`")
	s = strings.ReplaceAll(s, "$", "\\$")
	return s
}

func EscapeSed(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "/", "\\/")
	s = strings.ReplaceAll(s, "[", "\\[")
	s = strings.ReplaceAll(s, "]", "\\]")
	s = strings.ReplaceAll(s, ".", "\\.")
	s = strings.ReplaceAll(s, "^", "\\^")
	s = strings.ReplaceAll(s, "$", "\\$")
	s = strings.ReplaceAll(s, "*", "\\*")
	s = strings.ReplaceAll(s, "+", "\\+")
	s = strings.ReplaceAll(s, "?", "\\?")
	s = strings.ReplaceAll(s, "(", "\\(")
	s = strings.ReplaceAll(s, ")", "\\)")
	s = strings.ReplaceAll(s, "{", "\\{")
	s = strings.ReplaceAll(s, "}", "\\}")
	s = strings.ReplaceAll(s, "|", "\\|")
	return s
}

func EscapeSedReplacement(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "&", "\\&")
	s = strings.ReplaceAll(s, "/", "\\/")
	return s
}

func QuoteSingle(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}
