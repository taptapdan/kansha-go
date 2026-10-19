// Package extract reads a Go module's source and turns its doc comments into
// a plain data model that the renderer can turn into HTML.
//
// It uses the same machinery as go doc and pkg.go.dev: [packages.Load] finds
// the packages (respecting build tags and go.mod), [go/parser] reads the files,
// and [go/doc] groups declarations and attaches comments and examples.
package extract

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/doc"
	"go/parser"
	"go/printer"
	"go/token"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"golang.org/x/tools/go/packages"
)

// Options controls what Load includes.
type Options struct {
	// Dir is any directory inside the module. Defaults to ".".
	Dir string

	// Unexported includes unexported identifiers in library packages.
	// They are always included for package main.
	Unexported bool

	// Exclude reports whether a package directory (relative to the module
	// root) should be left out. May be nil.
	Exclude func(relDir string) bool
}

// Load reads every package in the module that contains opts.Dir.
func Load(opts Options) (*Module, error) {
	dir := opts.Dir
	if dir == "" {
		dir = "."
	}
	dir, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}

	cfg := &packages.Config{
		Mode: packages.NeedName | packages.NeedFiles | packages.NeedModule,
		Dir:  dir,
	}
	listed, err := packages.Load(cfg, "./...")
	if err != nil {
		return nil, fmt.Errorf("listing packages: %w", err)
	}

	mod := &Module{Dir: dir}
	for _, lp := range listed {
		if lp.Module != nil && mod.Path == "" {
			mod.Path = lp.Module.Path
			mod.Dir = lp.Module.Dir
		}
	}
	if mod.Path == "" {
		return nil, fmt.Errorf("no Go packages found in %s (is there a go.mod?)", dir)
	}

	for _, lp := range listed {
		if len(lp.GoFiles) == 0 {
			// Nothing to document, e.g. a directory with only test files.
			continue
		}
		rel, err := filepath.Rel(mod.Dir, lp.Dir)
		if err != nil {
			return nil, err
		}
		rel = filepath.ToSlash(rel)
		if opts.Exclude != nil && opts.Exclude(rel) {
			continue
		}
		pkg, err := loadPackage(mod.Dir, rel, lp, opts.Unexported)
		if err != nil {
			return nil, err
		}
		mod.Packages = append(mod.Packages, pkg)
	}

	sort.Slice(mod.Packages, func(i, j int) bool {
		return mod.Packages[i].RelDir < mod.Packages[j].RelDir
	})
	return mod, nil
}

// loadPackage parses one package's files, plus its _test.go files so that
// examples are picked up.
func loadPackage(root, rel string, lp *packages.Package, unexported bool) (*Package, error) {
	fset := token.NewFileSet()
	var files []*ast.File
	pkg := &Package{
		Name:       lp.Name,
		ImportPath: lp.PkgPath,
		RelDir:     rel,
		IsCommand:  lp.Name == "main",
	}

	for _, name := range lp.GoFiles {
		f, err := parser.ParseFile(fset, name, nil, parser.ParseComments)
		if err != nil {
			return nil, err
		}
		files = append(files, f)
		relFile, _ := filepath.Rel(root, name)
		pkg.Files = append(pkg.Files, filepath.ToSlash(relFile))
		pkg.Lines += fset.File(f.Pos()).LineCount()
	}

	tests, _ := filepath.Glob(filepath.Join(lp.Dir, "*_test.go"))
	for _, name := range tests {
		f, err := parser.ParseFile(fset, name, nil, parser.ParseComments)
		if err != nil {
			return nil, err
		}
		if f.Name.Name == lp.Name || f.Name.Name == lp.Name+"_test" {
			files = append(files, f)
		}
	}

	pkg.Unexported = unexported || pkg.IsCommand
	var mode doc.Mode
	if pkg.Unexported {
		mode |= doc.AllDecls
	}
	dp, err := doc.NewFromFiles(fset, files, lp.PkgPath, mode)
	if err != nil {
		return nil, err
	}
	pkg.docPkg = dp

	x := &extractor{fset: fset, root: root, files: files}

	pkg.Doc = dp.Doc
	pkg.Synopsis = dp.Synopsis(dp.Doc)
	pkg.DocPos = x.pos(files[0].Package)
	for _, f := range files {
		if f.Doc != nil && !strings.HasSuffix(fset.Position(f.Pos()).Filename, "_test.go") {
			pkg.DocPos = x.pos(f.Package)
			break
		}
	}

	pkg.Consts = x.values(dp.Consts)
	pkg.Vars = x.values(dp.Vars)
	pkg.Funcs = x.funcs(dp.Funcs)
	for _, t := range dp.Types {
		pkg.Types = append(pkg.Types, x.typ(t))
	}
	pkg.Examples = x.examples(dp.Examples)
	for _, note := range dp.Notes["BUG"] {
		pkg.Bugs = append(pkg.Bugs, note.Body)
	}
	return pkg, nil
}

type extractor struct {
	fset  *token.FileSet
	root  string
	files []*ast.File
}

func (x *extractor) pos(p token.Pos) Pos {
	position := x.fset.Position(p)
	rel, err := filepath.Rel(x.root, position.Filename)
	if err != nil {
		rel = position.Filename
	}
	return Pos{File: filepath.ToSlash(rel), Line: position.Line}
}

func (x *extractor) values(vals []*doc.Value) []*Value {
	var out []*Value
	for _, v := range vals {
		val := &Value{
			Names: v.Names,
			Doc:   v.Doc,
			Code:  x.genDecl(v.Decl),
			Pos:   x.pos(v.Decl.Pos()),
		}
		for _, spec := range v.Decl.Specs {
			vs, ok := spec.(*ast.ValueSpec)
			if !ok || vs.Doc != nil || vs.Comment != nil {
				continue
			}
			for _, n := range vs.Names {
				if n.Name != "_" {
					val.Undocumented = append(val.Undocumented, n.Name)
				}
			}
		}
		out = append(out, val)
	}
	return out
}

func (x *extractor) funcs(fns []*doc.Func) []*Func {
	var out []*Func
	for _, fn := range fns {
		f := &Func{
			Name:     fn.Name,
			Recv:     fn.Recv,
			RecvType: strings.TrimLeft(fn.Recv, "*"),
			Doc:      fn.Doc,
			Pos:      x.pos(fn.Decl.Pos()),
			Examples: x.examples(fn.Examples),
		}
		// Generic receivers look like "*List[T]"; link to the bare type name.
		if i := strings.IndexByte(f.RecvType, '['); i >= 0 {
			f.RecvType = f.RecvType[:i]
		}
		decl := *fn.Decl
		decl.Doc = nil
		decl.Body = nil
		f.Signature = x.print(&decl)
		out = append(out, f)
	}
	return out
}

func (x *extractor) typ(t *doc.Type) *Type {
	out := &Type{
		Name:     t.Name,
		Doc:      t.Doc,
		Kind:     "other",
		Pos:      x.pos(t.Decl.Pos()),
		Consts:   x.values(t.Consts),
		Vars:     x.values(t.Vars),
		Funcs:    x.funcs(t.Funcs),
		Methods:  x.funcs(t.Methods),
		Examples: x.examples(t.Examples),
	}
	out.Code = x.genDecl(t.Decl)

	for _, spec := range t.Decl.Specs {
		ts, ok := spec.(*ast.TypeSpec)
		if !ok || ts.Name.Name != t.Name {
			continue
		}
		out.Pos = x.pos(ts.Pos())
		switch st := ts.Type.(type) {
		case *ast.StructType:
			out.Kind = "struct"
			out.Hidden = st.Incomplete
			out.Fields = x.fields(st.Fields, false)
		case *ast.InterfaceType:
			out.Kind = "interface"
			out.Hidden = st.Incomplete
			out.Fields = x.fields(st.Methods, true)
		}
	}
	if out.Hidden {
		out.Code = markHidden(out.Code)
	}
	return out
}

const filteredNote = "// contains filtered or unexported fields"

// markHidden makes sure a struct or interface with removed unexported fields
// ends with exactly one "contains filtered or unexported fields" note, right
// after the last shown field. go/printer sometimes writes the note itself and
// sometimes only leaves a blank gap, depending on where the fields were.
func markHidden(code string) string {
	i := strings.LastIndex(code, "}")
	if i < 0 {
		return code
	}
	var kept []string
	for _, line := range strings.Split(code[:i], "\n") {
		if strings.TrimSpace(line) != filteredNote {
			kept = append(kept, line)
		}
	}
	body := strings.TrimRight(strings.Join(kept, "\n"), " \t\n")
	return body + "\n\t" + filteredNote + "\n" + code[i:]
}

func (x *extractor) fields(list *ast.FieldList, iface bool) []*Field {
	if list == nil {
		return nil
	}
	var out []*Field
	for _, fl := range list.List {
		f := &Field{Doc: strings.TrimSpace(fl.Doc.Text())}
		if f.Doc == "" {
			f.Doc = strings.TrimSpace(fl.Comment.Text())
		}
		var names []string
		for _, n := range fl.Names {
			names = append(names, n.Name)
		}
		switch {
		case len(names) == 0:
			f.Embedded = true
			f.Name = x.print(fl.Type)
		case iface:
			f.Name = strings.Join(names, ", ")
			f.Type = strings.TrimPrefix(x.print(fl.Type), "func")
		default:
			f.Name = strings.Join(names, ", ")
			f.Type = x.print(fl.Type)
		}
		out = append(out, f)
	}
	return out
}

// genDecl prints a const, var or type declaration together with the comments
// inside it (field comments, per-line comments in a const block), but without
// its own doc comment, which is rendered separately as prose.
func (x *extractor) genDecl(decl *ast.GenDecl) string {
	d := *decl
	d.Doc = nil

	var comments []*ast.CommentGroup
	add := func(cg *ast.CommentGroup) {
		if cg != nil {
			comments = append(comments, cg)
		}
	}
	single := len(d.Specs) == 1
	ast.Inspect(&d, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.Field:
			add(n.Doc)
			add(n.Comment)
		case *ast.ValueSpec:
			if !single {
				add(n.Doc)
			}
			add(n.Comment)
		case *ast.TypeSpec:
			if !single {
				add(n.Doc)
			}
			add(n.Comment)
		}
		return true
	})
	sort.Slice(comments, func(i, j int) bool { return comments[i].Pos() < comments[j].Pos() })

	// When there is a single spec, its doc comment is the one shown as prose.
	if single {
		if ts, ok := d.Specs[0].(*ast.TypeSpec); ok && ts.Doc != nil {
			spec := *ts
			spec.Doc = nil
			d.Specs = []ast.Spec{&spec}
		}
		if vs, ok := d.Specs[0].(*ast.ValueSpec); ok && vs.Doc != nil {
			spec := *vs
			spec.Doc = nil
			d.Specs = []ast.Spec{&spec}
		}
	}
	return x.print(&printer.CommentedNode{Node: &d, Comments: comments})
}

var outputRe = regexp.MustCompile(`(?im)^[ \t]*//[ \t]*(unordered )?output:`)

func (x *extractor) examples(exs []*doc.Example) []*Example {
	var out []*Example
	for _, ex := range exs {
		e := &Example{
			Name:      ex.Name,
			Suffix:    ex.Suffix,
			Doc:       ex.Doc,
			Output:    ex.Output,
			HasOutput: ex.Output != "" || ex.EmptyOutput,
			Unordered: ex.Unordered,
			File:      x.pos(ex.Code.Pos()).File,
		}

		code := x.print(&printer.CommentedNode{Node: ex.Code, Comments: ex.Comments})
		if _, ok := ex.Code.(*ast.BlockStmt); ok {
			code = unblock(code)
		}
		// The "// Output:" comment is shown separately below the code.
		if loc := outputRe.FindStringIndex(code); loc != nil {
			code = code[:loc[0]]
		}
		e.Code = strings.TrimRight(code, " \t\n")
		out = append(out, e)
	}
	return out
}

// unblock strips the braces around a printed block statement and removes one
// level of indentation.
func unblock(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "{")
	s = strings.TrimSuffix(s, "}")
	s = strings.Trim(s, "\n")
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		lines[i] = strings.TrimPrefix(l, "\t")
	}
	return strings.Join(lines, "\n")
}

func (x *extractor) print(node any) string {
	var buf bytes.Buffer
	cfg := printer.Config{Mode: printer.UseSpaces | printer.TabIndent, Tabwidth: 4}
	if err := cfg.Fprint(&buf, x.fset, node); err != nil {
		return fmt.Sprintf("/* %v */", err)
	}
	return buf.String()
}
