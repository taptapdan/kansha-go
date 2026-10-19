// Kansha builds a documentation site for a Go module from its doc comments.
//
// Usage:
//
//	kansha serve [-addr localhost:4000]   preview the docs, rebuilding on save
//	kansha build [-o dir]                 write the static site for GitHub Pages
//	kansha check                          list identifiers missing doc comments
//	kansha init                           add kansha.yaml and a Pages workflow
//
// Every command takes -C dir to run against a module in another directory.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"runtime/debug"
	"time"

	"github.com/taptapdan/kansha-go/internal/config"
	"github.com/taptapdan/kansha-go/internal/coverage"
	"github.com/taptapdan/kansha-go/internal/devserver"
	"github.com/taptapdan/kansha-go/internal/extract"
	"github.com/taptapdan/kansha-go/internal/render"
	"github.com/taptapdan/kansha-go/internal/scaffold"
	"github.com/taptapdan/kansha-go/internal/site"
)

// usage is printed for "kansha help" and when the command is missing.
const usage = `kansha builds documentation sites from Go doc comments.

Usage:
  kansha serve   Preview the docs at http://localhost:4000, rebuilding on save
  kansha build   Write the static site to _site/, or the output in kansha.yaml
  kansha check   List identifiers that are missing doc comments
  kansha init    Add kansha.yaml and a GitHub Pages workflow
  kansha version Print the version

Run "kansha <command> -h" for a command's flags.
`

// main dispatches to the subcommand named by the first argument.
func main() {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}
	cmd, args := os.Args[1], os.Args[2:]
	var err error
	switch cmd {
	case "serve":
		err = serve(args)
	case "build":
		err = build(args)
	case "check":
		err = check(args)
	case "init":
		err = initCmd(args)
	case "version", "-v", "--version":
		fmt.Println("kansha", version())
	case "help", "-h", "--help":
		fmt.Print(usage)
	default:
		fmt.Fprintf(os.Stderr, "kansha: unknown command %q\n\n%s", cmd, usage)
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "kansha:", err)
		os.Exit(1)
	}
}

// newFlags returns a flag set with the -C flag every command shares.
func newFlags(name string) (*flag.FlagSet, *string) {
	fs := flag.NewFlagSet("kansha "+name, flag.ExitOnError)
	dir := fs.String("C", ".", "run in `dir` instead of the current directory")
	return fs, dir
}

// serve runs the live-reloading docs server.
func serve(args []string) error {
	fs, dir := newFlags("serve")
	addr := fs.String("addr", "localhost:4000", "address to listen on")
	fs.Parse(args)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	return devserver.Run(ctx, site.Options{Dir: *dir}, *addr)
}

// build writes the static site to disk.
func build(args []string) error {
	fs, dir := newFlags("build")
	out := fs.String("o", "", "output `directory` (default: output in kansha.yaml, or _site)")
	showMissing := fs.Bool("show-missing", false, "include missing-doc callouts and the Coverage tab")
	fs.Parse(args)

	start := time.Now()
	cfg, err := config.Load(*dir)
	if err != nil {
		return err
	}
	outDir := *out
	if outDir == "" {
		outDir = cfg.Output
	}
	if !filepath.IsAbs(outDir) {
		outDir = filepath.Join(*dir, outDir)
	}
	r, err := render.New()
	if err != nil {
		return err
	}
	dst := site.DirOutput{Dir: outDir}
	if err := dst.Prepare(); err != nil {
		return err
	}
	res, err := site.Build(site.Options{Dir: *dir, ShowMissing: *showMissing}, r, dst)
	if err != nil {
		return err
	}
	// In GitHub Actions, tell later steps where the site is so the workflow
	// keeps working if output changes in kansha.yaml.
	if ghOut := os.Getenv("GITHUB_OUTPUT"); ghOut != "" {
		if f, err := os.OpenFile(ghOut, os.O_APPEND|os.O_WRONLY, 0o644); err == nil {
			fmt.Fprintf(f, "dir=%s\n", outDir)
			f.Close()
		}
	}
	st := res.Coverage.Stats
	fmt.Printf("Built %d pages into %s in %s\n", res.Pages, outDir, time.Since(start).Round(time.Millisecond))
	if st.Documented < st.Total {
		fmt.Printf("%d of %d identifiers are missing doc comments (run kansha check for the list)\n", st.Total-st.Documented, st.Total)
	}
	return nil
}

// check prints identifiers with missing or unconventional doc comments and
// exits with status 1 if any comments are missing.
func check(args []string) error {
	fs, dir := newFlags("check")
	fs.Parse(args)

	cfg, err := config.Load(*dir)
	if err != nil {
		return err
	}
	mod, err := extract.Load(extract.Options{Dir: *dir, Unexported: cfg.ShowUnexported, Exclude: cfg.Excluded})
	if err != nil {
		return err
	}
	rep := coverage.Analyze(mod)

	missing := 0
	for _, it := range rep.Items {
		tag := "tip    "
		if it.Status.Problem == coverage.Missing {
			tag = "missing"
			missing++
		}
		fmt.Printf("%s:%d: %s  %s\n", it.Pos.File, it.Pos.Line, tag, it.Message)
	}
	st := rep.Stats
	fmt.Printf("\n%d%% documented (%d of %d identifiers), %d style tips\n", st.Percent(), st.Documented, st.Total, st.Hints)
	if missing > 0 {
		os.Exit(1)
	}
	return nil
}

// initCmd adds kansha.yaml and the GitHub Pages workflow to a project.
func initCmd(args []string) error {
	fs, dir := newFlags("init")
	force := fs.Bool("force", false, "replace existing files")
	fs.Parse(args)

	fmt.Println("Setting up kansha:")
	if err := scaffold.Init(scaffold.Options{Dir: *dir, Force: *force, Log: os.Stdout}); err != nil {
		return err
	}
	fmt.Print(`
Next steps:
  go tool kansha serve     preview the docs at http://localhost:4000
  go tool kansha build     write the site to the output folder (_site/)

To publish on GitHub Pages, push to GitHub, then open Settings → Pages
and set "Source" to "GitHub Actions".
`)
	return nil
}

// version returns the module version kansha was built from.
func version() string {
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" {
		return info.Main.Version
	}
	return "(devel)"
}
