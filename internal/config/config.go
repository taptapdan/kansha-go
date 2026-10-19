// Package config loads the optional kansha.yaml file that sits at the root of
// a Go module and fills in sensible defaults for anything it leaves out.
//
// Every field is optional. A project with no kansha.yaml at all still gets a
// working site; the title falls back to the module name and the repository URL
// is read from the git remote named "origin".
package config

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// FileName is the name of the config file kansha looks for in the module root.
const FileName = "kansha.yaml"

// DefaultOutput is where kansha build writes the site when neither
// kansha.yaml nor the -o flag says otherwise. It matches the folder GitHub's
// upload-pages-artifact action uses by default.
const DefaultOutput = "_site"

// Config holds the settings from kansha.yaml.
type Config struct {
	// Title is shown in the top bar and the browser tab. Defaults to the last
	// element of the module path.
	Title string `yaml:"title"`

	// Description is a one-line summary shown on the home page.
	Description string `yaml:"description"`

	// Repo is the web URL of the repository, e.g.
	// https://github.com/taptapdan/go-tutorial. It is used for the GitHub link
	// in the top bar and for "view source" links on every symbol. Defaults to
	// the URL of the "origin" git remote.
	Repo string `yaml:"repo"`

	// Branch is the branch that source links point at. Defaults to "main".
	Branch string `yaml:"branch"`

	// ShowUnexported shows unexported (lowercase) identifiers in library
	// packages too: functions, methods, types, consts, vars and struct fields. Useful when the docs are for the people working on the code.
	// They are always shown for package main, since a command has no exported
	// API.
	ShowUnexported bool `yaml:"show_unexported"`

	// Unexported is the old name for ShowUnexported, still read so existing
	// kansha.yaml files keep working.
	Unexported bool `yaml:"unexported"`

	// ShowInstall shows a copyable "go install module@latest" command at the
	// top of each command's page (package main). Off by default, since it only
	// works once the module is published and is meant for tools other people
	// will install.
	ShowInstall bool `yaml:"show_install"`

	// Output is the folder kansha build writes the site to, relative to the
	// module root. Defaults to "_site". The -o flag overrides it.
	Output string `yaml:"output"`

	// Exclude lists package directories to leave out, relative to the module
	// root, e.g. "internal/scratch". A directory excludes everything below it.
	Exclude []string `yaml:"exclude"`
}

// Load reads kansha.yaml from dir if it exists and applies defaults.
func Load(dir string) (*Config, error) {
	cfg := &Config{}
	data, err := os.ReadFile(filepath.Join(dir, FileName))
	switch {
	case errors.Is(err, fs.ErrNotExist):
		// No config file is fine.
	case err != nil:
		return nil, err
	default:
		if err := yaml.Unmarshal(data, cfg); err != nil {
			return nil, fmt.Errorf("%s: %w", FileName, err)
		}
	}

	if cfg.Repo == "" {
		cfg.Repo = DetectRepo(dir)
	}
	cfg.Repo = strings.TrimSuffix(cfg.Repo, "/")
	if cfg.Branch == "" {
		cfg.Branch = "main"
	}
	if cfg.Output == "" {
		cfg.Output = DefaultOutput
	}
	if cfg.Unexported {
		cfg.ShowUnexported = true
	}
	return cfg, nil
}

// Excluded reports whether the package in relDir (slash-separated, relative to
// the module root) matches one of the Exclude entries.
func (c *Config) Excluded(relDir string) bool {
	for _, ex := range c.Exclude {
		ex = strings.Trim(filepath.ToSlash(ex), "/")
		if relDir == ex || strings.HasPrefix(relDir, ex+"/") {
			return true
		}
	}
	return false
}

// DetectRepo returns the web URL of the "origin" git remote in dir, or "" if
// there is none. SSH remotes such as git@github.com:user/repo.git are turned
// into https://github.com/user/repo.
func DetectRepo(dir string) string {
	out, err := exec.Command("git", "-C", dir, "remote", "get-url", "origin").Output()
	if err != nil {
		return ""
	}
	return RemoteToURL(strings.TrimSpace(string(out)))
}

// RemoteToURL converts a git remote address into a browsable https URL.
func RemoteToURL(remote string) string {
	u := strings.TrimSuffix(remote, ".git")
	switch {
	case strings.HasPrefix(u, "git@"):
		// git@github.com:user/repo
		u = strings.TrimPrefix(u, "git@")
		u = "https://" + strings.Replace(u, ":", "/", 1)
	case strings.HasPrefix(u, "ssh://git@"):
		u = "https://" + strings.TrimPrefix(u, "ssh://git@")
	case strings.HasPrefix(u, "http://"), strings.HasPrefix(u, "https://"):
	default:
		return ""
	}
	return u
}

// DetectBranch returns the branch currently checked out in dir, or "" if it
// cannot be determined.
func DetectBranch(dir string) string {
	out, err := exec.Command("git", "-C", dir, "symbolic-ref", "--short", "-q", "refs/remotes/origin/HEAD").Output()
	if err == nil {
		// origin/main -> main
		b := strings.TrimSpace(string(out))
		return strings.TrimPrefix(b, "origin/")
	}
	out, err = exec.Command("git", "-C", dir, "branch", "--show-current").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
