// Package config keeps the server URL and the personal token between
// runs, in $XDG_CONFIG_HOME/stopan/config.json (default
// ~/.config/stopan/config.json), readable only by the user.
// STOPAN_URL and STOPAN_TOKEN override the file.
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

type Config struct {
	URL   string `json:"url"`
	Token string `json:"token,omitempty"`
}

// Path is the config file.
func Path() (string, error) {
	dir := os.Getenv("XDG_CONFIG_HOME")
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		dir = filepath.Join(home, ".config")
	}
	return filepath.Join(dir, "stopan", "config.json"), nil
}

// Load reads the file, if any, and applies the environment. A file that
// others can read is refused, since it holds a token.
func Load() (Config, error) {
	var c Config
	p, err := Path()
	if err != nil {
		return c, err
	}
	if err := read(p, &c); err != nil {
		return c, err
	}
	if v := os.Getenv("STOPAN_URL"); v != "" {
		c.URL = v
	}
	if v := os.Getenv("STOPAN_TOKEN"); v != "" {
		c.Token = v
	}
	return c, nil
}

func read(p string, c *Config) error {
	fi, err := os.Stat(p)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if fi.Mode().Perm()&0o077 != 0 {
		return fmt.Errorf("%s is readable by other users; run: chmod 600 %s", p, p)
	}
	data, err := os.ReadFile(p)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(data, c); err != nil {
		return fmt.Errorf("%s: %w", p, err)
	}
	return nil
}

// Save writes c to the file (directory 0700, file 0600), replacing it
// in one rename. It never writes the environment's values unless the
// caller put them in c.
func Save(c Config) (string, error) {
	p, err := Path()
	if err != nil {
		return "", err
	}
	dir := filepath.Dir(p)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return "", err
	}
	f, err := os.CreateTemp(dir, ".config-*.json")
	if err != nil {
		return "", err
	}
	tmp := f.Name()
	if err = f.Chmod(0o600); err == nil {
		_, err = f.Write(append(data, '\n'))
	}
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		err = os.Rename(tmp, p)
	}
	if err != nil {
		os.Remove(tmp)
		return "", err
	}
	return p, nil
}

// FileOnly reads the file without the environment, so logout can keep
// the URL and drop only the token.
func FileOnly() (Config, error) {
	var c Config
	p, err := Path()
	if err != nil {
		return c, err
	}
	return c, read(p, &c)
}
