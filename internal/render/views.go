package render

import (
	"html/template"

	"github.com/taptapdan/kansha-go/internal/coverage"
)

// Site is the data shared by every page.
type Site struct {
	Title       string
	Description string
	ModulePath  string
	Repo        string // web URL of the repository, or ""
	Branch      string
	Dev         bool // served by kansha serve: adds live reload
	ShowMissing bool // show missing-doc callouts, coverage dots and the Coverage tab
	ShowInstall bool // show the "go install" command on command pages
	BuildID     string
	BuiltAt     string
	APIURL      string // first package page, linked from the API Reference tab
	Coverage    coverage.Stats
	Commands    []*NavPkg
	Libraries   []*NavPkg
}

// NavPkg is a package as listed in the sidebar and on the home page.
type NavPkg struct {
	Name       string
	Label      template.HTML // display path, e.g. "internal/<span>store</span>"
	ImportPath string
	URL        string // relative to the site root, e.g. "api/inventory/"
	Synopsis   string
	IsCommand  bool
	Stats      coverage.Stats
}

// Page is the data for one HTML page.
type Page struct {
	Site     *Site
	Path     string // output path, e.g. "api/inventory/index.html"
	Root     string // relative path back to the site root, e.g. "../../"
	URL      string // this page's URL relative to the root, e.g. "api/inventory/"
	Title    string
	Tab      string // "home", "api" or "coverage"
	TOC      []TOCItem
	Package  *PackageView
	Home     *HomeView
	Coverage *CoverageView
	Prev     *NavPkg
	Next     *NavPkg
}

// TOCItem is one entry in the "On this page" list.
type TOCItem struct {
	ID    string
	Text  string
	Level int  // 0 or 1
	Mono  bool // identifier, shown in the code font
	Warn  bool // has a missing doc comment
	Faint bool // deprecated
}

// PackageView is everything shown on a package page.
type PackageView struct {
	*NavPkg
	Doc        template.HTML
	DocStatus  coverage.Status
	DocPos     string // e.g. "inventory/doc.go"
	Files      int
	Lines      int
	Unexported bool
	Index      []IndexEntry
	Consts     []*Symbol
	Vars       []*Symbol
	Types      []*Symbol
	Funcs      []*Symbol
	Examples   []*ExampleView
	Bugs       []template.HTML
	Nav        []NavSymbol // symbols for the sidebar
	FirstIssue string      // anchor of the first missing comment
}

// IndexEntry is one line of a package's Index.
type IndexEntry struct {
	ID     string
	Code   template.HTML
	Indent bool
	Badge  string // "No docs", "Deprecated" or ""
}

// NavSymbol is a symbol listed under the active package in the sidebar.
type NavSymbol struct {
	ID         string
	Name       string
	Letter     string // T, F, M, C or V
	Kind       string
	Warn       bool
	Deprecated bool
	Indent     bool
}

// Symbol is a documented const, var, type, func or method.
type Symbol struct {
	ID         string
	ExtraIDs   []string // more anchors, for grouped consts and vars
	Kind       string   // "const", "var", "type", "func" or "method"
	Name       string
	Heading    template.HTML // highlighted, e.g. "func NewItem"
	Code       template.HTML // highlighted declaration or signature
	Doc        template.HTML
	Deprecated template.HTML
	Status     coverage.Status
	File       string // e.g. "inventory/restock.go"
	Line       int
	SourceURL  string
	Fields     []FieldView
	FieldLabel string // "Field" or "Method"
	Hidden     bool
	Examples   []*ExampleView
	Children   []*Symbol // constructors, methods and typed consts/vars of a type

	codeText string // Code before highlighting
}

// FieldView is a struct field or interface method row.
type FieldView struct {
	Name     string
	Type     string
	Doc      template.HTML
	Embedded bool
}

// ExampleView is a rendered example function.
type ExampleView struct {
	ID        string
	Title     string // "Example" or "Example (second)"
	FuncName  string // e.g. "ExampleItem_Total"
	File      string
	Doc       template.HTML
	Code      template.HTML
	Output    string
	HasOutput bool
	Unordered bool
}

// HomeView is the module home page.
type HomeView struct {
	Readme   template.HTML
	Packages []*NavPkg
}

// CoverageView is the Coverage tab.
type CoverageView struct {
	Packages []CoveragePkg
}

// CoveragePkg is one package's row on the Coverage tab.
type CoveragePkg struct {
	*NavPkg
	Items []CoverageItem
}

// CoverageItem is one identifier with a problem.
type CoverageItem struct {
	Kind     string
	Name     string
	URL      string
	Location string
	Problem  string
	Message  string
	Suggest  string
	Detail   string // for group tips: which names have no comment of their own
}
