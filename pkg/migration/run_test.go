package migration

import (
	"os"
	"testing"

	"github-to-codeberg/pkg/config"
)

func TestCleanupTempDirRemovesExistingTempDir(t *testing.T) {
	cfg := config.New()
	cfg.TempDir = t.TempDir()

	cleanupTempDir(cfg)

	if _, err := os.Stat(cfg.TempDir); !os.IsNotExist(err) {
		t.Fatalf("os.Stat(%q) error = %v want not exist", cfg.TempDir, err)
	}
}
