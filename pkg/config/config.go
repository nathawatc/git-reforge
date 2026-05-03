package config

import (
	"github.com/go-git/go-git/v5/plumbing"
)

type Config struct {
	SourceURL   string
	DestURL     string
	OldNames    []string
	NewName     string
	NewEmail    string
	Interactive bool
	SignMode    string
	GPGKey      string
	SSHKey      string
	SaveDir     string
	NoPush      bool

	// Co-authored-by configuration
	CoauthorMode     string
	CoauthorOldName  string
	CoauthorOldEmail string
	CoauthorNewName  string
	CoauthorNewEmail string

	// Interactive state — extracted to allow Config to be a pure config carrier
	State *InteractiveState

	// Internal
	TempDir string
	Logger  interface {
		Header(msg string)
		Info(msg string)
		Success(msg string)
		Warning(msg string)
		Error(msg string)
	}
}

type InteractiveState struct {
	CommitOverrides map[plumbing.Hash]CommitOverride
	CommitMessages  map[plumbing.Hash]string
	CoauthorActions map[plumbing.Hash]string
}

type CommitOverride struct {
	Name  string
	Email string
	Skip  bool
}

func New() *Config {
	return &Config{
		SignMode: "keep",
		State: &InteractiveState{
			CommitOverrides: make(map[plumbing.Hash]CommitOverride),
			CommitMessages:  make(map[plumbing.Hash]string),
			CoauthorActions: make(map[plumbing.Hash]string),
		},
	}
}

func (c *Config) WorkDir() string {
	return c.TempDir + "/working"
}
