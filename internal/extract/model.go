package extract

import "go/doc"

// Module is every documented package in one Go module.
type Module struct {
	Path     string // module path, e.g. "github.com/taptapdan/go-tutorial"
	Dir      string // absolute path of the module root
	Packages []*Package
}

// Package is the documentation for one Go package.
type Package struct {
	Name       string // package name from the package clause
	ImportPath string
	RelDir     string // slash-separated directory relative to the module root; "." for the root
	IsCommand  bool   // package main
	Unexported bool   // unexported identifiers are included

	Doc      string // package doc comment, raw text
	DocPos   Pos    // where the package clause is (in the file holding the doc comment, if any)
	Synopsis string // first sentence of Doc
	Files    []string
	Lines    int

	Consts   []*Value
	Vars     []*Value
	Funcs    []*Func // functions not attached to a type
	Types    []*Type
	Examples []*Example // package-level examples (func Example())
	Bugs     []string   // BUG(who): notes

	docPkg *doc.Package
}

// DocPackage returns the go/doc package, which knows how to parse and print
// this package's doc comments with links resolved.
func (p *Package) DocPackage() *doc.Package { return p.docPkg }

// Pos is a location in the source, relative to the module root.
type Pos struct {
	File string // slash-separated, e.g. "inventory/item.go"
	Line int
}

// Value is a const or var declaration. Grouped declarations
// (const ( A = 1; B = 2 )) are one Value with several names.
type Value struct {
	Names []string
	Doc   string
	Code  string // the declaration as Go source
	Pos   Pos
}

// Func is a function or method.
type Func struct {
	Name      string
	Recv      string // receiver as written, e.g. "*Item"; empty for functions
	RecvType  string // receiver type name without pointer, e.g. "Item"
	Doc       string
	Signature string // e.g. "func (i *Item) Total() int"
	Pos       Pos
	Examples  []*Example
}

// Type is a type declaration with everything go/doc associates with it:
// constructors, methods, and consts/vars of that type.
type Type struct {
	Name     string
	Doc      string
	Code     string // the declaration as Go source, with field comments
	Kind     string // "struct", "interface" or "other"
	Pos      Pos
	Fields   []*Field // struct fields or interface methods
	Hidden   bool     // some fields or methods were left out because they are unexported
	Consts   []*Value
	Vars     []*Value
	Funcs    []*Func // constructors: functions that return this type
	Methods  []*Func
	Examples []*Example
}

// Field is a struct field or an interface method.
type Field struct {
	Name     string // field names joined with ", ", or the embedded type
	Type     string // field type; for interface methods, the signature without "func"
	Doc      string
	Embedded bool
}

// Example is a testable example function from a _test.go file.
type Example struct {
	Name      string // e.g. "Item_Total"
	Suffix    string // e.g. "second" for ExampleItem_Total_second
	Doc       string
	Code      string
	Output    string
	HasOutput bool
	Unordered bool
	File      string
}
