package util

import "testing"

func TestSplitLines(t *testing.T) {
	tests := []struct {
		input    string
		expected int
	}{
		{"", 1},
		{"foo", 1},
		{"foo\nbar", 2},
		{"foo\nbar\nbaz", 3},
		{"foo\n\nbar", 3},
	}

	for _, tt := range tests {
		got := SplitLines(tt.input)
		if len(got) != tt.expected {
			t.Errorf("SplitLines(%q) = %d want %d", tt.input, len(got), tt.expected)
		}
	}
}

func TestSplitFirstLine(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"", ""},
		{"foo", "foo"},
		{"foo\nbar", "foo"},
		{"foo\nbar\nbaz", "foo"},
		{"\nbar", ""},
		{"foo\n", "foo"},
	}

	for _, tt := range tests {
		got := SplitFirstLine(tt.input)
		if got != tt.expected {
			t.Errorf("SplitFirstLine(%q) = %q want %q", tt.input, got, tt.expected)
		}
	}
}
