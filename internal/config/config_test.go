package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func setup(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("STOPAN_PROFILE", "")
	t.Setenv("STOPAN_URL", "")
	t.Setenv("STOPAN_TOKEN", "")
	return filepath.Join(dir, "stopan", "config.json")
}

func TestSaveRead(t *testing.T) {
	path := setup(t)
	if _, _, err := Load(""); err == nil || !strings.Contains(err.Error(), "not logged in") {
		t.Fatalf("no file: %v", err)
	}
	f, _ := Read()
	f.Profiles["prod"] = &Profile{URL: "https://desk.example.org", Token: "stopan_prod", User: "Ben (agent)"}
	f.Profiles["dev"] = &Profile{URL: "http://localhost:3000", Token: "stopan_dev"}
	f.Current = "prod"
	p, err := f.Save()
	if err != nil || p != path {
		t.Fatalf("save: %s %v", p, err)
	}
	for name, want := range map[string]os.FileMode{p: 0o600, filepath.Dir(p): 0o700} {
		if fi, err := os.Stat(name); err != nil || fi.Mode().Perm() != want {
			t.Errorf("%s mode = %v, want %v", name, fi.Mode().Perm(), want)
		}
	}
	g, err := Read()
	if err != nil || g.Current != "prod" || strings.Join(g.Names(), ",") != "dev,prod" || g.Profiles["prod"].User != "Ben (agent)" {
		t.Fatalf("read = %+v, %v", g, err)
	}
}

func TestChooseAndOverride(t *testing.T) {
	setup(t)
	f, _ := Read()
	f.Profiles["prod"] = &Profile{URL: "https://desk.example.org", Token: "stopan_prod"}
	f.Profiles["dev"] = &Profile{URL: "http://localhost:3000", Token: "stopan_dev"}
	f.Current = "prod"
	f.Save()

	check := func(name, wantName, wantURL, wantToken string) {
		t.Helper()
		p, n, err := Load(name)
		if err != nil || n != wantName || p.URL != wantURL || p.Token != wantToken {
			t.Errorf("Load(%q) = %+v %q %v; want %s %s %s", name, p, n, err, wantName, wantURL, wantToken)
		}
	}
	check("", "prod", "https://desk.example.org", "stopan_prod")
	check("dev", "dev", "http://localhost:3000", "stopan_dev")
	t.Setenv("STOPAN_PROFILE", "dev")
	check("", "dev", "http://localhost:3000", "stopan_dev")
	check("prod", "prod", "https://desk.example.org", "stopan_prod") // -P wins over the environment
	t.Setenv("STOPAN_TOKEN", "stopan_env")
	check("", "dev", "http://localhost:3000", "stopan_env")

	if _, _, err := Load("nope"); err == nil || !strings.Contains(err.Error(), `no profile "nope"`) {
		t.Errorf("missing profile: %v", err)
	}
	t.Setenv("STOPAN_URL", "https://other.example.org")
	check("nope", "nope", "https://other.example.org", "stopan_env") // the environment alone is enough
	if _, _, err := Load("../x"); err == nil {
		t.Error("a bad name was accepted")
	}
}

func TestOldFormat(t *testing.T) {
	path := setup(t)
	os.MkdirAll(filepath.Dir(path), 0o700)
	os.WriteFile(path, []byte(`{"url":"https://desk.example.org","token":"stopan_old"}`), 0o600)
	p, n, err := Load("")
	if err != nil || n != "default" || p.Token != "stopan_old" {
		t.Fatalf("old file: %+v %q %v", p, n, err)
	}
	f, _ := Read()
	f.Save()
	data, _ := os.ReadFile(path)
	if !strings.Contains(string(data), `"profiles"`) || !strings.Contains(string(data), `"current": "default"`) {
		t.Errorf("saved as:\n%s", data)
	}
}

func TestRemove(t *testing.T) {
	setup(t)
	f, _ := Read()
	f.Profiles["a"] = &Profile{URL: "https://a.example.org"}
	f.Profiles["b"] = &Profile{URL: "https://b.example.org"}
	f.Current = "a"
	f.Remove("a")
	if f.Current != "b" || len(f.Profiles) != 1 {
		t.Errorf("after remove: %+v", f)
	}
	f.Remove("b")
	if f.Current != "" {
		t.Errorf("current = %q", f.Current)
	}
}

func TestReadableByOthersRefused(t *testing.T) {
	setup(t)
	f, _ := Read()
	f.Profiles["x"] = &Profile{URL: "https://x.example.org", Token: "t"}
	p, err := f.Save()
	if err != nil {
		t.Fatal(err)
	}
	os.Chmod(p, 0o644)
	if _, err := Read(); err == nil || !strings.Contains(err.Error(), "chmod 600") {
		t.Errorf("err = %v", err)
	}
}

func TestValidName(t *testing.T) {
	for _, ok := range []string{"default", "dev", "prod-2", "a.b_c"} {
		if ValidName(ok) != nil {
			t.Errorf("%q refused", ok)
		}
	}
	for _, bad := range []string{"", "-x", "../x", "a b", strings.Repeat("x", 33)} {
		if ValidName(bad) == nil {
			t.Errorf("%q accepted", bad)
		}
	}
}
