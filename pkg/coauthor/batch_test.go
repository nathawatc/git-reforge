package coauthor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-git/go-git/v5/plumbing"

	"github-to-codeberg/pkg/config"
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

func TestBuildCoauthorMsgFilterEscapesReplacementOnlyAsReplacement(t *testing.T) {
	cfg := config.New()
	cfg.TempDir = t.TempDir()
	cfg.CoauthorMode = "rewrite"
	cfg.CoauthorOldName = "Alice"
	cfg.CoauthorNewName = "New.Name & Co"
	cfg.CoauthorNewEmail = "new/name@example.org"
	outputPath := filepath.Join(cfg.TempDir, "coauthor_msg_filter.sh")

	if err := buildCoauthorMsgFilter(cfg, outputPath, "batch"); err != nil {
		t.Fatalf("buildCoauthorMsgFilter() error = %v", err)
	}

	contents, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatal(err)
	}
	script := string(contents)
	if !strings.Contains(script, `Co-authored-by: New.Name \& Co <new\/name@example.org>`) {
		t.Fatalf("script does not contain correctly escaped replacement:\n%s", script)
	}
	if strings.Contains(script, `New\.Name`) || strings.Contains(script, `example\.org`) {
		t.Fatalf("script escaped replacement text as a regex pattern:\n%s", script)
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

func TestApplyCoauthorActionAppliesAllRemaining(t *testing.T) {
	cfg := config.New()
	commits := []string{
		"1111111111111111111111111111111111111111",
		"2222222222222222222222222222222222222222",
		"3333333333333333333333333333333333333333",
	}

	applyCoauthorAction(commits, "rewrite", cfg)

	for _, commit := range commits {
		hash := plumbing.NewHash(commit)
		if got := cfg.State.CoauthorActions[hash]; got != "rewrite" {
			t.Fatalf("CoauthorActions[%s] = %q want rewrite", commit, got)
		}
	}
}
