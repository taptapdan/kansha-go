// Package scaffold implements kansha init: it adds a starter kansha.yaml and a
// GitHub Actions workflow that publishes the docs to GitHub Pages.
package scaffold

import (
	"embed"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/taptapdan/kansha-go/internal/config"
)

//go:embed files
var files embed.FS

// Options controls Init.
type Options struct {
	Dir   string    // module root
	Force bool      // overwrite existing files
	Log   io.Writer // progress messages
}

// Init writes the starter files into opts.Dir. Existing files are left alone
// unless opts.Force is set.
func Init(opts Options) error {
	title := filepath.Base(mustAbs(opts.Dir))
	repo := config.DetectRepo(opts.Dir)
	branch := config.DetectBranch(opts.Dir)
	if branch == "" {
		branch = "main"
	}

	repl := strings.NewReplacer(
		"__TITLE__", title,
		"__REPO__", repo,
		"__BRANCH__", branch,
	)

	targets := []struct{ src, dst string }{
		{"files/kansha.yaml", config.FileName},
		{"files/docs.yml", filepath.Join(".github", "workflows", "docs.yml")},
	}
	for _, t := range targets {
		dst := filepath.Join(opts.Dir, t.dst)
		if _, err := os.Stat(dst); err == nil && !opts.Force {
			fmt.Fprintf(opts.Log, "  skip    %s (already exists; use -force to replace)\n", t.dst)
			continue
		}
		b, err := fs.ReadFile(files, t.src)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(dst, []byte(repl.Replace(string(b))), 0o644); err != nil {
			return err
		}
		fmt.Fprintf(opts.Log, "  create  %s\n", t.dst)
	}

	// Ignore the output folder, honoring a kansha.yaml that already existed.
	cfg, err := config.Load(opts.Dir)
	if err != nil {
		return err
	}
	out := strings.Trim(filepath.ToSlash(cfg.Output), "/") + "/"
	if err := ignore(filepath.Join(opts.Dir, ".gitignore"), out); err != nil {
		return err
	}
	fmt.Fprintf(opts.Log, "  update  .gitignore (ignore %s)\n", out)
	return nil
}

// ignore adds line to a .gitignore file unless it is already there.
func ignore(path, line string) error {
	b, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	for _, l := range strings.Split(string(b), "\n") {
		if strings.TrimSpace(l) == line || strings.TrimSpace(l) == strings.TrimSuffix(line, "/") {
			return nil
		}
	}
	s := string(b)
	if s != "" && !strings.HasSuffix(s, "\n") {
		s += "\n"
	}
	s += line + "\n"
	return os.WriteFile(path, []byte(s), 0o644)
}

func mustAbs(p string) string {
	a, err := filepath.Abs(p)
	if err != nil {
		return p
	}
	return a
}
