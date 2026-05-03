package util

import "strings"

func SplitLines(s string) []string {
	return strings.Split(s, "\n")
}

func SplitFirstLine(s string) string {
	if i := strings.Index(s, "\n"); i >= 0 {
		return s[:i]
	}
	return s
}
