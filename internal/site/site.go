// Package site builds the complete documentation site: one page per package,
// a home page, a coverage page, a search index and the static assets.
//
// Pages are written through an [Output], so the same build can go to disk
// (kansha build) or stay in memory (kansha serve).
package site

import (
	"encoding/json"
	"fmt"
	"html/template"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/taptapdan/kansha-go/internal/config"
	"github.com/taptapdan/kansha-go/internal/coverage"
	"github.com/taptapdan/kansha-go/internal/extract"
	"github.com/taptapdan/kansha-go/internal/render"
	"github.com/taptapdan/kansha-go/web"
)

// Options controls a build.
type Options struct {
	Dir         string // any directory inside the module
	Dev         bool   // add live reload (kansha serve)
	ShowMissing bool   // show missing-doc callouts and the Coverage tab
}

// Output receives the built files. Paths are slash-separated and relative to
// the site root, e.g. "api/inventory/index.html".
type Output interface {
	Write(path string, data []byte) error
}

// Result describes a finished build.
type Result struct {
	Module   *extract.Module
	Coverage *coverage.Report
	Pages    int
	BuildID  string
}

// Build loads the module in opts.Dir and writes the whole site to out.
func Build(opts Options, r *render.Renderer, out Output) (*Result, error) {
	cfg, err := config.Load(opts.Dir)
	if err != nil {
		return nil, err
	}
	mod, err := extract.Load(extract.Options{
		Dir:        opts.Dir,
		Unexported: cfg.Unexported,
		Exclude:    cfg.Excluded,
	})
	if err != nil {
		return nil, err
	}
	report := coverage.Analyze(mod)

	now := time.Now()
	site := &render.Site{
		Title:       cfg.Title,
		Description: cfg.Description,
		ModulePath:  mod.Path,
		Repo:        cfg.Repo,
		Branch:      cfg.Branch,
		Dev:         opts.Dev,
		ShowMissing: opts.ShowMissing,
		BuildID:     strconv.FormatInt(now.UnixNano(), 36),
		Coverage:    report.Stats,
	}
	if site.Title == "" {
		site.Title = path.Base(mod.Path)
	}
	if !opts.Dev {
		site.BuiltAt = "built " + now.Format("2 Jan 2006, 15:04")
	}

	// Navigation entries, and import path -> URL for cross-package links.
	urls := map[string]string{}
	navs := map[*extract.Package]*render.NavPkg{}
	var ordered []*render.NavPkg
	for _, p := range mod.Packages {
		n := &render.NavPkg{
			Name:       p.Name,
			Label:      label(mod.Path, p),
			ImportPath: p.ImportPath,
			URL:        render.PackageURL(mod.Path, p),
			Synopsis:   p.Synopsis,
			IsCommand:  p.IsCommand,
			Stats:      report.Pkgs[p],
		}
		navs[p] = n
		urls[p.ImportPath] = n.URL
		if p.IsCommand {
			site.Commands = append(site.Commands, n)
		} else {
			site.Libraries = append(site.Libraries, n)
		}
	}
	// Commands are listed first in the sidebar, so page order matches.
	ordered = append(append(ordered, site.Commands...), site.Libraries...)
	if len(site.Libraries) > 0 {
		site.APIURL = site.Libraries[0].URL
	} else if len(ordered) > 0 {
		site.APIURL = ordered[0].URL
	}

	res := &Result{Module: mod, Coverage: report, BuildID: site.BuildID}
	write := func(name string, p *render.Page) error {
		html, err := r.Render(name, p)
		if err != nil {
			return err
		}
		res.Pages++
		return out.Write(p.Path, html)
	}

	var search []searchEntry

	// Package pages.
	byURL := map[string]*extract.Package{}
	for _, p := range mod.Packages {
		byURL[navs[p].URL] = p
	}
	for i, n := range ordered {
		p := byURL[n.URL]
		page := &render.Page{
			Site:  site,
			Path:  n.URL + "index.html",
			URL:   n.URL,
			Title: p.Name,
			Tab:   "api",
		}
		if p.IsCommand {
			page.Title = path.Base(strings.TrimSuffix(n.URL, "/"))
		}
		page.Root = render.RootFor(page.Path)
		if i > 0 {
			page.Prev = ordered[i-1]
		}
		if i < len(ordered)-1 {
			page.Next = ordered[i+1]
		}
		page.Package, page.TOC = render.BuildPackage(render.PackageInput{
			Site: site, Nav: n, Pkg: p, Root: page.Root, URLs: urls,
		})
		if err := write("package", page); err != nil {
			return nil, err
		}
		search = append(search, searchEntries(page.Package)...)
	}

	// Home page, with the README if there is one.
	home := &render.Page{Site: site, Path: "index.html", Tab: "home", Home: &render.HomeView{Packages: ordered}}
	if readme, err := os.ReadFile(filepath.Join(mod.Dir, "README.md")); err == nil {
		html, err := r.Markdown(readme)
		if err != nil {
			return nil, fmt.Errorf("README.md: %w", err)
		}
		home.Home.Readme = html
	}
	if err := write("home", home); err != nil {
		return nil, err
	}

	// Coverage page.
	if opts.ShowMissing {
		cov := &render.Page{Site: site, Path: "coverage/index.html", URL: "coverage/", Title: "Doc coverage", Tab: "coverage", Coverage: &render.CoverageView{}}
		cov.Root = render.RootFor(cov.Path)
		for _, n := range ordered {
			cp := render.CoveragePkg{NavPkg: n}
			for _, it := range report.Items {
				if navs[it.Pkg] != n {
					continue
				}
				cp.Items = append(cp.Items, render.CoverageItem{
					Kind:     it.Kind,
					Name:     it.Name,
					URL:      n.URL + "#" + it.Anchor,
					Location: fmt.Sprintf("%s:%d", it.Pos.File, it.Pos.Line),
					Problem:  it.Status.Problem,
					Message:  it.Message,
					Suggest:  it.Status.Suggest,
				})
			}
			cov.Coverage.Packages = append(cov.Coverage.Packages, cp)
		}
		if err := write("coverage", cov); err != nil {
			return nil, err
		}
	}

	// Search index.
	data, err := json.Marshal(search)
	if err != nil {
		return nil, err
	}
	if err := out.Write("search.json", data); err != nil {
		return nil, err
	}

	// Static assets.
	assets := map[string]string{
		"assets/kansha.css": "dist/kansha.css",
		"assets/kansha.js":  "js/kansha.js",
	}
	for dst, src := range assets {
		b, err := fs.ReadFile(web.FS, src)
		if err != nil {
			return nil, err
		}
		if err := out.Write(dst, b); err != nil {
			return nil, err
		}
	}
	// Tells GitHub Pages to serve the files as-is instead of running Jekyll.
	if err := out.Write(".nojekyll", nil); err != nil {
		return nil, err
	}
	return res, nil
}

// label is how a package is named in navigation: its directory relative to the
// module root, with the parent path dimmed.
func label(modPath string, p *extract.Package) template.HTML {
	rel := p.RelDir
	if rel == "." {
		if p.IsCommand {
			return template.HTML(template.HTMLEscapeString(path.Base(modPath)))
		}
		return template.HTML(template.HTMLEscapeString(p.Name))
	}
	dir, base := path.Split(rel)
	if dir == "" {
		return template.HTML(template.HTMLEscapeString(base))
	}
	return template.HTML(`<span class="text-faint">` + template.HTMLEscapeString(dir) + `</span>` + template.HTMLEscapeString(base))
}

type searchEntry struct {
	Name    string `json:"name"`
	Kind    string `json:"kind"`
	Pkg     string `json:"pkg"`
	URL     string `json:"url"`
	Summary string `json:"summary,omitempty"`
}

func searchEntries(v *render.PackageView) []searchEntry {
	pkg := v.ImportPath
	out := []searchEntry{{Name: v.Name, Kind: "package", Pkg: pkg, URL: v.URL, Summary: v.Synopsis}}
	if v.IsCommand {
		out[0].Kind = "command"
		out[0].Name = path.Base(strings.TrimSuffix(v.URL, "/"))
	}
	var add func(s *render.Symbol)
	add = func(s *render.Symbol) {
		out = append(out, searchEntry{Name: s.Name, Kind: s.Kind, Pkg: pkg, URL: v.URL + "#" + s.ID})
		for _, c := range s.Children {
			add(c)
		}
	}
	for _, group := range [][]*render.Symbol{v.Consts, v.Vars, v.Types, v.Funcs} {
		for _, s := range group {
			add(s)
		}
	}
	return out
}
