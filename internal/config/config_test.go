package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSaveLoad(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("STOPAN_URL", "")
	t.Setenv("STOPAN_TOKEN", "")

	if c, err := Load(); err != nil || c != (Config{}) {
		t.Fatalf("no file: %+v, %v", c, err)
	}
	p, err := Save(Config{URL: "https://desk.example.org", Token: "stopan_abc"})
	if err != nil {
		t.Fatal(err)
	}
	if p != filepath.Join(dir, "stopan", "config.json") {
		t.Errorf("path = %s", p)
	}
	for path, want := range map[string]os.FileMode{p: 0o600, filepath.Dir(p): 0o700} {
		fi, err := os.Stat(path)
		if err != nil || fi.Mode().Perm() != want {
			t.Errorf("%s mode = %v, want %v", path, fi.Mode().Perm(), want)
		}
	}
	c, err := Load()
	if err != nil || c.URL != "https://desk.example.org" || c.Token != "stopan_abc" {
		t.Fatalf("load = %+v, %v", c, err)
	}

	t.Setenv("STOPAN_TOKEN", "stopan_env")
	if c, _ := Load(); c.Token != "stopan_env" || c.URL != "https://desk.example.org" {
		t.Errorf("env override = %+v", c)
	}
	if c, _ := FileOnly(); c.Token != "stopan_abc" {
		t.Errorf("FileOnly = %+v", c)
	}
}

func TestReadableByOthersRefused(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	p, err := Save(Config{URL: "https://x.org", Token: "t"})
	if err != nil {
		t.Fatal(err)
	}
	os.Chmod(p, 0o644)
	if _, err := Load(); err == nil || !strings.Contains(err.Error(), "chmod 600") {
		t.Errorf("err = %v", err)
	}
}
