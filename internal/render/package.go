package render

import (
	"fmt"
	"go/doc"
	"go/doc/comment"
	"html/template"
	"regexp"
	"strings"

	"github.com/taptapdan/kansha-go/internal/coverage"
	"github.com/taptapdan/kansha-go/internal/extract"
)

// PackageURL returns the page URL for p relative to the site root, e.g.
// "api/inventory/". A package at the module root uses the module's last path
// element.
func PackageURL(modPath string, p *extract.Package) string {
	rel := p.RelDir
	if rel == "." {
		rel = modPath[strings.LastIndexByte(modPath, '/')+1:]
	}
	return "api/" + rel + "/"
}

// PackageInput is what BuildPackage needs to know about the rest of the site.
type PackageInput struct {
	Site *Site
	Nav  *NavPkg
	Pkg  *extract.Package
	Root string            // relative path from the package page to the site root
	URLs map[string]string // import path -> page URL, for links to other packages
}

// BuildPackage converts an extracted package into a PackageView and the
// entries for its "On this page" list.
func BuildPackage(in PackageInput) (*PackageView, []TOCItem) {
	c := &pkgCtx{in: in, dp: in.Pkg.DocPackage()}
	p := in.Pkg
	v := &PackageView{
		NavPkg:     in.Nav,
		DocStatus:  coverage.CheckPackage(p),
		DocPos:     p.DocPos.File,
		Files:      len(p.Files),
		Lines:      p.Lines,
		Unexported: p.Unexported,
	}

	var toc []TOCItem
	doc, headings := c.docWithHeadings(p.Doc)
	v.Doc = doc
	toc = append(toc, TOCItem{ID: "overview", Text: "Overview", Warn: !v.DocStatus.OK() && v.DocStatus.Problem == coverage.Missing})
	toc = append(toc, headings...)

	for _, val := range p.Consts {
		v.Consts = append(v.Consts, c.value("const", val))
	}
	for _, val := range p.Vars {
		v.Vars = append(v.Vars, c.value("var", val))
	}
	for _, t := range p.Types {
		v.Types = append(v.Types, c.typ(t))
	}
	for _, f := range p.Funcs {
		v.Funcs = append(v.Funcs, c.fn(f))
	}
	for _, ex := range p.Examples {
		v.Examples = append(v.Examples, c.example(ex, "example"))
	}
	for _, b := range p.Bugs {
		v.Bugs = append(v.Bugs, c.doc(b))
	}

	if len(v.Consts)+len(v.Vars)+len(v.Types)+len(v.Funcs) > 0 {
		toc = append(toc, TOCItem{ID: "index", Text: "Index"})
	}
	if len(v.Examples) > 0 {
		toc = append(toc, TOCItem{ID: "examples", Text: "Examples"})
	}
	if len(v.Consts) > 0 {
		toc = append(toc, TOCItem{ID: "constants", Text: "Constants"})
	}
	if len(v.Vars) > 0 {
		toc = append(toc, TOCItem{ID: "variables", Text: "Variables"})
	}

	// Index, sidebar and the rest of the TOC follow page order.
	addIndex := func(s *Symbol, indent bool) {
		e := IndexEntry{ID: s.ID, Indent: indent, Code: s.Heading}
		switch s.Kind {
		case "func", "method":
			e.Code = highlight(oneLine(s.codeText))
		}
		switch {
		case s.Deprecated != "":
			e.Badge = "Deprecated"
		case s.Status.Problem == coverage.Missing && in.Site.ShowMissing:
			e.Badge = "No docs"
		}
		v.Index = append(v.Index, e)
	}
	nav := func(s *Symbol, indent bool) {
		ns := NavSymbol{
			ID: s.ID, Name: s.Name, Kind: s.Kind, Indent: indent,
			Warn:       s.Status.Problem == coverage.Missing && in.Site.ShowMissing,
			Deprecated: s.Deprecated != "",
		}
		ns.Letter = map[string]string{"type": "T", "func": "F", "method": "M", "const": "C", "var": "V"}[s.Kind]
		v.Nav = append(v.Nav, ns)
	}
	tocSym := func(s *Symbol, level int) {
		toc = append(toc, TOCItem{
			ID: s.ID, Text: s.Name, Level: level, Mono: true,
			Warn:  s.Status.Problem == coverage.Missing && in.Site.ShowMissing,
			Faint: s.Deprecated != "",
		})
	}
	for _, s := range v.Consts {
		addIndex(s, false)
	}
	for _, s := range v.Vars {
		addIndex(s, false)
	}
	if len(v.Types) > 0 {
		toc = append(toc, TOCItem{ID: "types", Text: "Types"})
	}
	for _, s := range v.Types {
		addIndex(s, false)
		nav(s, false)
		tocSym(s, 1)
		for _, child := range s.Children {
			if child.Kind == "func" || child.Kind == "method" {
				addIndex(child, true)
				nav(child, true)
				tocSym(child, 1)
			}
		}
	}
	if len(v.Funcs) > 0 {
		toc = append(toc, TOCItem{ID: "functions", Text: "Functions"})
	}
	for _, s := range v.Funcs {
		addIndex(s, false)
		nav(s, false)
		tocSym(s, 1)
	}
	if len(v.Bugs) > 0 {
		toc = append(toc, TOCItem{ID: "bugs", Text: "Known bugs"})
	}

	if in.Site.ShowMissing {
		coverage.Walk(p, func(kind, name, anchor string, pos extract.Pos, s coverage.Status) {
			if v.FirstIssue == "" && s.Problem == coverage.Missing {
				v.FirstIssue = anchor
			}
		})
	}
	return v, toc
}

type pkgCtx struct {
	in PackageInput
	dp *doc.Package
}

func (c *pkgCtx) value(kind string, v *extract.Value) *Symbol {
	s := &Symbol{
		ID:     v.Names[0],
		Kind:   kind,
		Name:   strings.Join(v.Names, ", "),
		Status: coverage.CheckValue(v),
	}
	if len(v.Names) > 1 {
		s.ExtraIDs = v.Names[1:]
	}
	s.Heading = highlight(kind + " " + s.Name)
	c.common(s, v.Doc, v.Code, v.Pos)
	return s
}

func (c *pkgCtx) typ(t *extract.Type) *Symbol {
	s := &Symbol{
		ID:     t.Name,
		Kind:   "type",
		Name:   t.Name,
		Status: coverage.Check(t.Name, t.Doc),
		Hidden: t.Hidden,
	}
	s.Heading = highlight("type " + t.Name)
	c.common(s, t.Doc, t.Code, t.Pos)

	if t.Kind == "interface" {
		s.FieldLabel = "Method"
	} else {
		s.FieldLabel = "Field"
	}
	for _, f := range t.Fields {
		s.Fields = append(s.Fields, FieldView{Name: f.Name, Type: f.Type, Doc: c.inline(f.Doc), Embedded: f.Embedded})
	}
	for _, ex := range t.Examples {
		s.Examples = append(s.Examples, c.example(ex, "example-"+t.Name))
	}
	for _, v := range t.Consts {
		s.Children = append(s.Children, c.value("const", v))
	}
	for _, v := range t.Vars {
		s.Children = append(s.Children, c.value("var", v))
	}
	for _, f := range t.Funcs {
		s.Children = append(s.Children, c.fn(f))
	}
	for _, f := range t.Methods {
		s.Children = append(s.Children, c.fn(f))
	}
	return s
}

func (c *pkgCtx) fn(f *extract.Func) *Symbol {
	s := &Symbol{ID: f.Name, Kind: "func", Name: f.Name, Status: coverage.Check(f.Name, f.Doc)}
	if f.Recv != "" {
		s.Kind = "method"
		s.ID = f.RecvType + "." + f.Name
		s.Name = s.ID
		s.Heading = highlight("func (" + f.Recv + ") " + f.Name)
	} else {
		s.Heading = highlight("func " + f.Name)
	}
	c.common(s, f.Doc, f.Signature, f.Pos)
	for _, ex := range f.Examples {
		s.Examples = append(s.Examples, c.example(ex, "example-"+s.ID))
	}
	return s
}

// common fills in the doc, code and source link shared by every symbol.
func (c *pkgCtx) common(s *Symbol, docText, code string, pos extract.Pos) {
	rest, dep := splitDeprecated(docText)
	s.Doc = c.doc(rest)
	if dep != "" {
		s.Deprecated = c.inline(dep)
	}
	s.codeText = code
	s.Code = highlight(code)
	s.File = pos.File
	s.Line = pos.Line
	s.SourceURL = c.sourceURL(pos)
}

func (c *pkgCtx) sourceURL(pos extract.Pos) string {
	site := c.in.Site
	if site.Repo == "" || pos.File == "" {
		return ""
	}
	return fmt.Sprintf("%s/blob/%s/%s#L%d", site.Repo, site.Branch, pos.File, pos.Line)
}

func (c *pkgCtx) example(ex *extract.Example, idPrefix string) *ExampleView {
	v := &ExampleView{
		ID:        idPrefix,
		Title:     "Example",
		FuncName:  "Example" + ex.Name,
		File:      ex.File,
		Doc:       c.doc(ex.Doc),
		Code:      highlight(ex.Code),
		Output:    ex.Output,
		HasOutput: ex.HasOutput,
		Unordered: ex.Unordered,
	}
	if ex.Suffix != "" {
		v.ID += "-" + ex.Suffix
		v.Title += " (" + ex.Suffix + ")"
		v.FuncName += "_" + ex.Suffix
	}
	return v
}

// printer returns a go/doc/comment printer that links identifiers to this
// page, to other packages in the module, or to pkg.go.dev.
func (c *pkgCtx) printer(headingLevel int) *comment.Printer {
	pr := c.dp.Printer()
	pr.HeadingLevel = headingLevel
	pr.HeadingID = func(h *comment.Heading) string { return "hdr-" + slug(plainText(h.Text)) }
	pr.DocLinkURL = func(link *comment.DocLink) string {
		anchor := link.Name
		if link.Recv != "" {
			anchor = link.Recv + "." + link.Name
		}
		frag := ""
		if anchor != "" {
			frag = "#" + anchor
		}
		if link.ImportPath == "" || link.ImportPath == c.in.Pkg.ImportPath {
			return frag
		}
		if u, ok := c.in.URLs[link.ImportPath]; ok {
			return c.in.Root + u + frag
		}
		return "https://pkg.go.dev/" + link.ImportPath + frag
	}
	return pr
}

func (c *pkgCtx) doc(text string) template.HTML {
	if strings.TrimSpace(text) == "" {
		return ""
	}
	d := c.dp.Parser().Parse(text)
	return template.HTML(highlightPre(string(c.printer(3).HTML(d))))
}

// docWithHeadings renders the package doc and returns its # headings for the
// table of contents.
func (c *pkgCtx) docWithHeadings(text string) (template.HTML, []TOCItem) {
	if strings.TrimSpace(text) == "" {
		return "", nil
	}
	d := c.dp.Parser().Parse(text)
	var toc []TOCItem
	for _, b := range d.Content {
		if h, ok := b.(*comment.Heading); ok {
			t := plainText(h.Text)
			toc = append(toc, TOCItem{ID: "hdr-" + slug(t), Text: t, Level: 1})
		}
	}
	return template.HTML(highlightPre(string(c.printer(2).HTML(d)))), toc
}

// inline renders a short comment without the surrounding <p>.
func (c *pkgCtx) inline(text string) template.HTML {
	h := strings.TrimSpace(string(c.doc(text)))
	if strings.HasPrefix(h, "<p>") && strings.HasSuffix(h, "</p>") && strings.Count(h, "<p>") == 1 {
		h = strings.TrimSuffix(strings.TrimPrefix(h, "<p>"), "</p>")
	}
	return template.HTML(h)
}

// splitDeprecated pulls the "Deprecated: ..." paragraph out of a doc comment
// so it can be shown as a callout.
func splitDeprecated(text string) (rest, deprecated string) {
	paras := strings.Split(text, "\n\n")
	for i, p := range paras {
		if strings.HasPrefix(p, "Deprecated: ") {
			deprecated = strings.TrimPrefix(p, "Deprecated: ")
			paras = append(paras[:i], paras[i+1:]...)
			break
		}
	}
	return strings.Join(paras, "\n\n"), deprecated
}

func plainText(texts []comment.Text) string {
	var b strings.Builder
	for _, t := range texts {
		switch t := t.(type) {
		case comment.Plain:
			b.WriteString(string(t))
		case comment.Italic:
			b.WriteString(string(t))
		case *comment.Link:
			b.WriteString(plainText(t.Text))
		case *comment.DocLink:
			b.WriteString(plainText(t.Text))
		}
	}
	return b.String()
}

var nonSlug = regexp.MustCompile(`[^a-z0-9]+`)

func slug(s string) string {
	return strings.Trim(nonSlug.ReplaceAllString(strings.ToLower(s), "-"), "-")
}

var spaces = regexp.MustCompile(`\s+`)

func oneLine(s string) string {
	return spaces.ReplaceAllString(strings.TrimSpace(s), " ")
}
