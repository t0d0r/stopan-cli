// Package config keeps named profiles, each a server URL and a personal
// token, in $XDG_CONFIG_HOME/stopan/config.json (default
// ~/.config/stopan/config.json), readable only by the user. One profile
// is current. STOPAN_PROFILE picks another; STOPAN_URL and STOPAN_TOKEN
// override the chosen profile's values.
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
)

// DefaultName is the profile used when none is named and none exists.
const DefaultName = "default"

// Profile is one server and token.
type Profile struct {
	URL   string `json:"url"`
	Token string `json:"token,omitempty"`
	User  string `json:"user,omitempty"` // "NAME (ROLE)" from the last login, for the list
}

// File is the whole config file.
type File struct {
	Current  string              `json:"current,omitempty"`
	Profiles map[string]*Profile `json:"profiles"`
}

var nameRE = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,31}$`)

// ValidName tells whether s may name a profile.
func ValidName(s string) error {
	if !nameRE.MatchString(s) {
		return fmt.Errorf("%q is not a profile name (letters, digits, '.', '_', '-', at most 32)", s)
	}
	return nil
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

// Read reads the file; a missing file is an empty one. The format of
// v0.1.0 (one url and token at the top) becomes the profile "default".
// A file that others can read is refused, since it holds tokens.
func Read() (*File, error) {
	f := &File{Profiles: map[string]*Profile{}}
	p, err := Path()
	if err != nil {
		return nil, err
	}
	fi, err := os.Stat(p)
	if errors.Is(err, fs.ErrNotExist) {
		return f, nil
	}
	if err != nil {
		return nil, err
	}
	if fi.Mode().Perm()&0o077 != 0 {
		return nil, fmt.Errorf("%s is readable by other users; run: chmod 600 %s", p, p)
	}
	data, err := os.ReadFile(p)
	if err != nil {
		return nil, err
	}
	var raw struct {
		File
		URL   string `json:"url"`
		Token string `json:"token"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("%s: %w", p, err)
	}
	if raw.Profiles != nil {
		f.Profiles = raw.Profiles
	}
	f.Current = raw.Current
	if raw.URL != "" && f.Profiles[DefaultName] == nil {
		f.Profiles[DefaultName] = &Profile{URL: raw.URL, Token: raw.Token}
		if f.Current == "" {
			f.Current = DefaultName
		}
	}
	for n, pr := range f.Profiles {
		if pr == nil {
			delete(f.Profiles, n)
		}
	}
	return f, nil
}

// Names are the profile names in order.
func (f *File) Names() []string {
	names := make([]string, 0, len(f.Profiles))
	for n := range f.Profiles {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// Remove deletes a profile; when it was current, the first remaining
// one becomes current.
func (f *File) Remove(name string) {
	delete(f.Profiles, name)
	if f.Current == name {
		f.Current = ""
		if names := f.Names(); len(names) > 0 {
			f.Current = names[0]
		}
	}
}

// Save writes the file (directory 0700, file 0600), replacing it in one
// rename.
func (f *File) Save() (string, error) {
	p, err := Path()
	if err != nil {
		return "", err
	}
	dir := filepath.Dir(p)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	data, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return "", err
	}
	tmp, err := os.CreateTemp(dir, ".config-*.json")
	if err != nil {
		return "", err
	}
	if err = tmp.Chmod(0o600); err == nil {
		_, err = tmp.Write(append(data, '\n'))
	}
	if cerr := tmp.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		err = os.Rename(tmp.Name(), p)
	}
	if err != nil {
		os.Remove(tmp.Name())
		return "", err
	}
	return p, nil
}

// Choose names the profile to use: the given name (the -P option), else
// STOPAN_PROFILE, else the current one, else the only one, else
// "default".
func (f *File) Choose(name string) string {
	switch {
	case name != "":
		return name
	case os.Getenv("STOPAN_PROFILE") != "":
		return os.Getenv("STOPAN_PROFILE")
	case f.Current != "":
		return f.Current
	case len(f.Profiles) == 1:
		return f.Names()[0]
	}
	return DefaultName
}

// Load returns the profile to use and its name, with STOPAN_URL and
// STOPAN_TOKEN applied. A named profile that does not exist is an error
// unless the environment gives both URL and token.
func Load(name string) (Profile, string, error) {
	f, err := Read()
	if err != nil {
		return Profile{}, "", err
	}
	n := f.Choose(name)
	if err := ValidName(n); err != nil {
		return Profile{}, "", err
	}
	var p Profile
	if saved := f.Profiles[n]; saved != nil {
		p = *saved
	}
	if v := os.Getenv("STOPAN_URL"); v != "" {
		p.URL = v
	}
	if v := os.Getenv("STOPAN_TOKEN"); v != "" {
		p.Token = v
	}
	if f.Profiles[n] == nil && (p.URL == "" || p.Token == "") {
		if len(f.Profiles) == 0 {
			return p, n, errors.New("not logged in; run 'stopan login'")
		}
		return p, n, fmt.Errorf("no profile %q; run 'stopan login -P %s' or 'stopan profiles'", n, n)
	}
	return p, n, nil
}
