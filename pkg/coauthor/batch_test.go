package coauthor

import (
	"strings"
	"testing"

	"github-to-codeberg/pkg/shell"
)

func TestEscapeSed(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"simple", "simple"},
		{"with/slash", `with\/slash`},
		{"with[brackets]", `with\[brackets\]`},
		{"with.dot", `with\.dot`},
	}

	for _, tt := range tests {
		got := shell.EscapeSed(tt.input)
		if got != tt.expected {
			t.Errorf("EscapeSed(%q) = %q want %q", tt.input, got, tt.expected)
		}
	}
}

func TestMatchesCoauthor(t *testing.T) {
	tests := []struct {
		name          string
		body          string
		coauthorName  string
		coauthorEmail string
		want          bool
	}{
		{
			name:          "exact match with email",
			body:          "Fix bug\n\nCo-authored-by: Alice <alice@example.org>",
			coauthorName:  "Alice",
			coauthorEmail: "alice@example.org",
			want:          true,
		},
		{
			name:          "no match wrong email",
			body:          "Co-authored-by: Alice <alice@other.org>",
			coauthorName:  "Alice",
			coauthorEmail: "alice@example.org",
			want:          false,
		},
		{
			name:          "no match wrong name",
			body:          "Co-authored-by: Bob <bob@example.org>",
			coauthorName:  "Alice",
			coauthorEmail: "alice@example.org",
			want:          false,
		},
		{
			name:          "name only match",
			body:          "Co-authored-by: Alice <alice@example.org>",
			coauthorName:  "Alice",
			coauthorEmail: "",
			want:          true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := matchesCoauthor(tt.body, tt.coauthorName, tt.coauthorEmail)
			if got != tt.want {
				t.Errorf("matchesCoauthor() = %v want %v", got, tt.want)
			}
		})
	}
}

func TestExtractMatchingLines(t *testing.T) {
	body := "Fix bug\n\nCo-authored-by: Alice <alice@example.org>\nCo-authored-by: Bob <bob@example.org>\n"

	got := extractMatchingLines(body, "Alice", "alice@example.org")
	if len(got) != 1 {
		t.Errorf("got %d lines want 1", len(got))
	}
	if !strings.Contains(got[0], "Alice") {
		t.Errorf("got %q want Alice", got)
	}

	got2 := extractMatchingLines(body, "Bob", "")
	if len(got2) != 1 {
		t.Errorf("got %d lines want 1", len(got2))
	}
}
