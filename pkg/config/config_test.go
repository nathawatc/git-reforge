package config

import (
	"testing"
)

func TestNew(t *testing.T) {
	cfg := New()

	if cfg.SignMode != "keep" {
		t.Errorf("SignMode = %q want %q", cfg.SignMode, "keep")
	}
	if cfg.Logger == nil {
		t.Fatal("Logger is nil")
	}
	if cfg.State == nil {
		t.Fatal("State is nil")
	}
	if cfg.State.CommitOverrides == nil {
		t.Error("State.CommitOverrides is nil")
	}
	if cfg.State.CommitMessages == nil {
		t.Error("State.CommitMessages is nil")
	}
	if cfg.State.CoauthorActions == nil {
		t.Error("State.CoauthorActions is nil")
	}
}

func TestCommitOverride(t *testing.T) {
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
}

func TestInteractiveState(t *testing.T) {
	state := &InteractiveState{
		CommitOverrides: make(map[string]CommitOverride),
		CommitMessages:  make(map[string]string),
		CoauthorActions: make(map[string]string),
	}

	hash := "abc123def4560000000000000000000000000000"
	state.CommitOverrides[hash] = CommitOverride{Name: "Test", Email: "test@example.org"}
	state.CommitMessages[hash] = "New message"
	state.CoauthorActions[hash] = "rewrite"

	if state.CommitOverrides[hash].Name != "Test" {
		t.Errorf("CommitOverrides[hash].Name = %q", state.CommitOverrides[hash].Name)
	}
	if state.CommitMessages[hash] != "New message" {
		t.Errorf("CommitMessages[hash] = %q", state.CommitMessages[hash])
	}
	if state.CoauthorActions[hash] != "rewrite" {
		t.Errorf("CoauthorActions[hash] = %q", state.CoauthorActions[hash])
	}
}
