package shell

import "strings"

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
