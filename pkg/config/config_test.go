package config

import (
	"testing"

	"github.com/go-git/go-git/v5/plumbing"
)

func TestNew(t *testing.T) {
	cfg := New()

	if cfg.SignMode != "keep" {
		t.Errorf("SignMode = %q want %q", cfg.SignMode, "keep")
	}
	if cfg.CommitOverrides == nil {
		t.Error("CommitOverrides is nil")
	}
	if cfg.CommitMessages == nil {
		t.Error("CommitMessages is nil")
	}
	if cfg.CoauthorActions == nil {
		t.Error("CoauthorActions is nil")
	}
}

func TestCommitOverride(t *testing.T) {
	hash := plumbing.NewHash("abc123def456")
	override := CommitOverride{
		Name:  "Test User",
		Email: "test@example.org",
		Skip:  false,
	}

	if override.Name != "Test User" {
		t.Errorf("Name = %q want %q", override.Name, "Test User")
	}
	if override.Email != "test@example.org" {
		t.Errorf("Email = %q want %q", override.Email, "test@example.org")
	}
	if override.Skip {
		t.Error("Skip = true want false")
	}
	if hash.String() != "abc123def4560000000000000000000000000000" {
		t.Errorf("Hash = %q", hash.String())
	}
}
