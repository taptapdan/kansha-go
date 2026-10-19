// Package render turns the extracted documentation into HTML pages, using the
// templates and styles embedded from the web folder.
package render

import (
	"bytes"
	"fmt"
	"html/template"
	"io/fs"
	"path"
	"strings"

	"github.com/yuin/goldmark"
	highlighting "github.com/yuin/goldmark-highlighting/v2"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"

	chromahtml "github.com/alecthomas/chroma/v2/formatters/html"

	"github.com/taptapdan/kansha-go/web"
)

// Renderer executes the page templates.
type Renderer struct {
	pages map[string]*template.Template
	md    goldmark.Markdown
}

// New parses the embedded templates.
func New() (*Renderer, error) {
	tfs, err := fs.Sub(web.FS, "templates")
	if err != nil {
		return nil, err
	}
	base, err := template.New("base").Funcs(funcs).ParseFS(tfs, "layout.html", "partials.html")
	if err != nil {
		return nil, err
	}

	r := &Renderer{pages: map[string]*template.Template{}}
	for _, name := range []string{"home", "package", "coverage"} {
		t, err := base.Clone()
		if err != nil {
			return nil, err
		}
		if _, err := t.ParseFS(tfs, name+".html"); err != nil {
			return nil, err
		}
		r.pages[name] = t
	}

	r.md = goldmark.New(
		goldmark.WithExtensions(
			extension.GFM,
			highlighting.NewHighlighting(
				highlighting.WithFormatOptions(chromahtml.WithClasses(true)),
			),
		),
		goldmark.WithParserOptions(parser.WithAutoHeadingID()),
	)
	return r, nil
}

// Render executes the template for p.Tab ("home", "package" or "coverage").
func (r *Renderer) Render(name string, p *Page) ([]byte, error) {
	t, ok := r.pages[name]
	if !ok {
		return nil, fmt.Errorf("render: unknown page %q", name)
	}
	var buf bytes.Buffer
	if err := t.ExecuteTemplate(&buf, "layout", p); err != nil {
		return nil, fmt.Errorf("render %s: %w", p.Path, err)
	}
	return buf.Bytes(), nil
}

// Markdown converts Markdown to HTML.
func (r *Renderer) Markdown(src []byte) (template.HTML, error) {
	var buf bytes.Buffer
	if err := r.md.Convert(src, &buf); err != nil {
		return "", err
	}
	return template.HTML(buf.String()), nil
}

// RootFor returns the relative path from a page at outPath back to the site
// root: "" for "index.html", "../../" for "api/inventory/index.html".
func RootFor(outPath string) string {
	depth := strings.Count(path.Dir(outPath), "/")
	if path.Dir(outPath) == "." {
		return ""
	}
	return strings.Repeat("../", depth+1)
}

var funcs = template.FuncMap{
	// pctClass picks a color for a coverage percentage.
	"pctClass": func(pct int) string {
		switch {
		case pct >= 100:
			return "text-ok"
		case pct >= 60:
			return "text-warn"
		default:
			return "text-danger"
		}
	},
	"barClass": func(pct int) string {
		switch {
		case pct >= 100:
			return "bg-ok"
		case pct >= 60:
			return "bg-warn"
		default:
			return "bg-danger"
		}
	},
	"base": path.Base,
	// navArgs and symArgs bundle the page with an item, since a template call
	// takes only one argument.
	"navArgs": func(p *Page, pkg *NavPkg) any {
		return struct {
			Page *Page
			Pkg  *NavPkg
		}{p, pkg}
	},
	"symArgs": func(p *Page, s *Symbol) any {
		return struct {
			Page *Page
			Sym  *Symbol
		}{p, s}
	},
	"plural": func(n int, one, many string) string {
		if n == 1 {
			return fmt.Sprintf("%d %s", n, one)
		}
		return fmt.Sprintf("%d %s", n, many)
	},
}
