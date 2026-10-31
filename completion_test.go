package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"stopan-cli/internal/config"
)

func TestCompletionScripts(t *testing.T) {
	for _, sh := range shells {
		code, out, errs := runCLI(t, "", "completion", sh)
		if code != 0 {
			t.Fatalf("%s: %d %q", sh, code, errs)
		}
		for _, c := range compCommands() {
			if !strings.Contains(out, c.name) {
				t.Errorf("%s script lacks the command %s", sh, c.name)
			}
			for _, o := range c.opts {
				if !strings.Contains(out, "--"+o.long) && !(sh == "fish" && strings.Contains(out, "-l "+o.long)) {
					t.Errorf("%s script lacks %s --%s", sh, c.name, o.long)
				}
			}
		}
	}
	if code, _, _ := runCLI(t, "", "completion", "tcsh"); code != 2 {
		t.Errorf("unknown shell: exit %d", code)
	}
	if code, _, _ := runCLI(t, "", "completion"); code != 2 {
		t.Errorf("no shell: exit %d", code)
	}
}

// shellEnv builds stopan into a temporary PATH and saves two profiles,
// so the scripts can call 'stopan profiles --names'.
func shellEnv(t *testing.T, shell string) (dir string, environ []string) {
	t.Helper()
	if _, err := exec.LookPath(shell); err != nil {
		t.Skipf("%s not installed", shell)
	}
	dir = t.TempDir()
	if out, err := exec.Command("go", "build", "-o", filepath.Join(dir, "stopan"), ".").CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, out)
	}
	cfg := filepath.Join(dir, "cfg")
	t.Setenv("XDG_CONFIG_HOME", cfg)
	f, _ := config.Read()
	f.Profiles["prod"] = &config.Profile{URL: "https://desk.example.org", Token: "stopan_test"}
	f.Profiles["dev"] = &config.Profile{URL: "http://localhost:3000", Token: "stopan_test"}
	if _, err := f.Save(); err != nil {
		t.Fatal(err)
	}
	var script strings.Builder
	writeCompletion(&script, shell)
	os.WriteFile(filepath.Join(dir, "script"), []byte(script.String()), 0o600)
	return dir, append(os.Environ(), "PATH="+dir+":"+os.Getenv("PATH"), "XDG_CONFIG_HOME="+cfg)
}

var completionCases = []struct{ line, want string }{
	{"stopan li", "list"},
	{"stopan re", "reply read revoke"},
	{"stopan list ", "unsolved mine unassigned pending postponed done all open"},
	{"stopan ls -v p", "pending postponed"},
	{"stopan list --st", "--status"},
	{"stopan list mine ", ""},
	{"stopan -P ", "dev prod"},
	{"stopan -P dev list -s d", "done"},
	{"stopan use ", "dev prod"},
	{"stopan set 1001 -p ", "low normal high urgent"},
	{"stopan set 1001 -a ", "me none"},
	{"stopan read -", "-a --all -P --profile -h --help"},
	{"stopan completion ", "bash zsh fish"},
	{"stopan help no", "note notifications"},
	{"stopan reply 1001 --profile ", "dev prod"},
}

func TestBashCompletion(t *testing.T) {
	dir, environ := shellEnv(t, "bash")
	var drv strings.Builder
	drv.WriteString("source " + filepath.Join(dir, "script") + "\n")
	drv.WriteString(`comp() { read -ra COMP_WORDS <<< "$1"; [[ $1 == *" " ]] && COMP_WORDS+=(""); COMP_CWORD=$((${#COMP_WORDS[@]} - 1)); _stopan; echo "${COMPREPLY[*]}"; }` + "\n")
	for _, c := range completionCases {
		drv.WriteString("comp '" + c.line + "'\n")
	}
	cmd := exec.Command("bash", "--norc", "--noprofile", "-c", drv.String())
	cmd.Env = environ
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("bash: %v\n%s", err, out)
	}
	lines := strings.Split(strings.TrimSuffix(string(out), "\n"), "\n")
	if len(lines) != len(completionCases) {
		t.Fatalf("got %d lines for %d cases:\n%s", len(lines), len(completionCases), out)
	}
	for i, c := range completionCases {
		if lines[i] != c.want {
			t.Errorf("bash %q = %q, want %q", c.line, lines[i], c.want)
		}
	}
}

func TestFishCompletion(t *testing.T) {
	dir, environ := shellEnv(t, "fish")
	for _, c := range completionCases {
		if c.want == "" {
			continue // fish offers the search words' files and views freely
		}
		cmd := exec.Command("fish", "--no-config", "-c", "source "+filepath.Join(dir, "script")+"; complete -C '"+c.line+"'")
		cmd.Env = environ
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("fish: %v\n%s", err, out)
		}
		var got []string
		for _, l := range strings.Split(strings.TrimSpace(string(out)), "\n") {
			got = append(got, strings.SplitN(l, "\t", 2)[0])
		}
		for _, w := range strings.Fields(c.want) {
			if !strings.Contains(" "+strings.Join(got, " ")+" ", " "+w+" ") {
				t.Errorf("fish %q = %v, missing %s", c.line, got, w)
			}
		}
	}
}

func TestZshCompletionSyntax(t *testing.T) {
	dir, environ := shellEnv(t, "zsh")
	cmd := exec.Command("zsh", "-n", filepath.Join(dir, "script"))
	cmd.Env = environ
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Errorf("zsh -n: %v\n%s", err, out)
	}
}
