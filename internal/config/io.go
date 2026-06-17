package config

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// baseDir returns $envVar if set, else $HOME/fallbackSubdir.
func baseDir(envVar, fallbackSubdir string) (string, error) {
	if v := os.Getenv(envVar); v != "" {
		return v, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, fallbackSubdir), nil
}

// ClaudeDir returns the Claude Code base directory: ${CLAUDE_CONFIG_DIR:-$HOME/.claude}.
// It is the parent of the skills/ and rules/ destinations herdle init writes, and
// of the projects/ dir scanned for seeding.
func ClaudeDir() (string, error) {
	return baseDir("CLAUDE_CONFIG_DIR", ".claude")
}

// Path returns the config file location: $HERDLE_CONFIG if set, else
// ${XDG_CONFIG_HOME:-$HOME/.config}/herdle/config.toml.
func Path() (string, error) {
	if v := os.Getenv("HERDLE_CONFIG"); v != "" {
		return v, nil
	}
	base, err := baseDir("XDG_CONFIG_HOME", ".config")
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "herdle", "config.toml"), nil
}

// Load reads the config from Path(). A missing file is a valid empty config.
func Load() (*Config, error) {
	p, err := Path()
	if err != nil {
		return nil, err
	}
	return LoadFrom(p)
}

// LoadFrom reads the config from an explicit path. A missing file yields an empty
// config and no error.
func LoadFrom(path string) (*Config, error) {
	var c Config
	_, err := toml.DecodeFile(path, &c) // #nosec G304 -- path is the user's configured herdle config
	if errors.Is(err, fs.ErrNotExist) {
		return &Config{}, nil
	}
	if err != nil {
		return nil, err
	}
	// Migrate any legacy `gh =` slug into the forge-agnostic Slug so an upgraded
	// config does not silently lose PR correlation.
	for i := range c.Projects {
		c.Projects[i].foldLegacyGH()
	}
	return &c, nil
}

// Save writes the config to Path().
func (c *Config) Save() error {
	p, err := Path()
	if err != nil {
		return err
	}
	return c.SaveTo(p)
}

// SaveTo writes the config atomically (temp file + rename) to an explicit path,
// creating the parent directory if needed.
func (c *Config) SaveTo(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".config-*.toml")
	if err != nil {
		return err
	}
	if err := toml.NewEncoder(tmp).Encode(c); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmp.Name())
		return err
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmp.Name())
		return err
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		_ = os.Remove(tmp.Name())
		return err
	}
	return nil
}
