package coauthor

import (
	"strings"
	"testing"

	"git-reforge/pkg/config"
)

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

func TestExtractMatchingLinesForTargets(t *testing.T) {
	body := "Fix bug\n\nCo-authored-by: Alice <alice@example.org>\nCo-authored-by: Bob <bob@example.org>\n"

	got := extractMatchingLinesForTargets(body, []coauthorTarget{{Name: "Alice", Email: "alice@example.org"}})
	if len(got) != 1 {
		t.Errorf("got %d lines want 1", len(got))
	}
	if !strings.Contains(got[0], "Alice") {
		t.Errorf("got %q want Alice", got)
	}

	got2 := extractMatchingLinesForTargets(body, []coauthorTarget{{Name: "Bob"}})
	if len(got2) != 1 {
		t.Errorf("got %d lines want 1", len(got2))
	}
}

func TestApplyCoauthorActionAppliesAllRemaining(t *testing.T) {
	cfg := config.New()
	commits := []string{
		"1111111111111111111111111111111111111111",
		"2222222222222222222222222222222222222222",
		"3333333333333333333333333333333333333333",
	}

	applyCoauthorAction(commits, "rewrite", cfg)

	for _, commit := range commits {
		if got := cfg.State.CoauthorActions[commit]; got != "rewrite" {
			t.Fatalf("CoauthorActions[%s] = %q want rewrite", commit, got)
		}
	}
}
