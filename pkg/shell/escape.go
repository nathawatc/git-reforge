package shell

import "strings"

func EscapeShell(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "\"", "\\\"")
	s = strings.ReplaceAll(s, "`", "\\`")
	s = strings.ReplaceAll(s, "$", "\\$")
	s = strings.ReplaceAll(s, "'", "'\\''")
	return s
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
