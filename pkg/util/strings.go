package util

import "strings"

func SplitLines(s string) []string {
	return strings.Split(s, "\n")
}

func SplitFirstLine(s string) string {
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			return s[:i]
		}
	}
	return s
}
