// Package coverage finds identifiers that are missing doc comments, or whose
// comments don't follow the Go convention of starting with the identifier's
// name.
//
// A missing comment counts against a package's coverage. A comment that
// doesn't start with the name is only a hint: the docs still render, but
// tools like pkg.go.dev and go doc read better when the convention is followed.
package coverage

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/taptapdan/kansha-go/internal/extract"
)

// Problem kinds.
const (
	Missing = "missing" // no doc comment at all
	Style   = "style"   // comment doesn't start with the identifier's name
)

// Status is the result of checking one identifier's doc comment.
type Status struct {
	Problem string // "", Missing or Style
	Suggest string // the comment we'd expect it to start with, e.g. "// Restock ..."

	// For const and var groups with more than one name: the names that have
	// no comment of their own, and the group's size.
	Group        bool
	Undocumented []string
	GroupSize    int
}

// OK reports whether there is nothing to flag.
func (s Status) OK() bool { return s.Problem == "" }

// Item is one identifier with a problem.
type Item struct {
	Pkg     *extract.Package
	Kind    string // "package", "func", "method", "type", "const" or "var"
	Name    string // e.g. "Restock" or "Item.Total"
	Anchor  string // id of the element on the package page
	Pos     extract.Pos
	Status  Status
	Message string
}

// Stats counts documented identifiers.
type Stats struct {
	Total      int
	Documented int
	Hints      int
}

// Percent returns the share of documented identifiers, from 0 to 100.
func (s Stats) Percent() int {
	if s.Total == 0 {
		return 100
	}
	return s.Documented * 100 / s.Total
}

// Report is the result of checking a whole module.
type Report struct {
	Stats Stats
	Pkgs  map[*extract.Package]Stats
	Items []Item // missing comments first, then style hints, in source order
}

// Analyze checks every identifier in m.
func Analyze(m *extract.Module) *Report {
	r := &Report{Pkgs: map[*extract.Package]Stats{}}
	var missing, hints []Item
	for _, p := range m.Packages {
		var st Stats
		visit := func(kind, name, anchor string, pos extract.Pos, s Status) {
			st.Total++
			if s.Problem != Missing {
				st.Documented++
			}
			if s.Problem == "" {
				return
			}
			it := Item{Pkg: p, Kind: kind, Name: name, Anchor: anchor, Pos: pos, Status: s}
			if s.Problem == Missing {
				it.Message = fmt.Sprintf("%s %s has no doc comment", kind, name)
				if s.Group {
					it.Message = fmt.Sprintf("%s group: %s %s no comment", kind, strings.Join(s.Undocumented, ", "), hasHave(len(s.Undocumented)))
				}
				missing = append(missing, it)
			} else {
				st.Hints++
				it.Message = fmt.Sprintf("comment for %s should start with %q", name, strings.TrimPrefix(s.Suggest, "// "))
				if s.Group {
					it.Message = fmt.Sprintf("%s group: %s (the group comment covers %s)",
						kind, OwnCommentDetail(s.Undocumented), themIt(len(s.Undocumented)))
				}
				hints = append(hints, it)
			}
		}
		Walk(p, visit)
		r.Pkgs[p] = st
		r.Stats.Total += st.Total
		r.Stats.Documented += st.Documented
		r.Stats.Hints += st.Hints
	}
	r.Items = append(missing, hints...)
	return r
}

// Walk calls fn for every documentable identifier in p, in page order, with
// the result of checking its comment.
func Walk(p *extract.Package, fn func(kind, name, anchor string, pos extract.Pos, s Status)) {
	fn("package", p.Name, "overview", p.DocPos, CheckPackage(p))
	values := func(kind string, vals []*extract.Value) {
		for _, v := range vals {
			fn(kind, strings.Join(v.Names, ", "), v.Names[0], v.Pos, CheckValue(v))
		}
	}
	funcs := func(fns []*extract.Func) {
		for _, f := range fns {
			kind, name := "func", f.Name
			if f.Recv != "" {
				kind, name = "method", f.RecvType+"."+f.Name
			}
			fn(kind, name, name, f.Pos, Check(f.Name, f.Doc))
		}
	}
	values("const", p.Consts)
	values("var", p.Vars)
	for _, t := range p.Types {
		fn("type", t.Name, t.Name, t.Pos, Check(t.Name, t.Doc))
		values("const", t.Consts)
		values("var", t.Vars)
		funcs(t.Funcs)
		funcs(t.Methods)
	}
	funcs(p.Funcs)
}

// CheckPackage checks the package doc comment, which by convention starts with
// "Package name". Commands (package main) are only checked for a comment.
func CheckPackage(p *extract.Package) Status {
	if strings.TrimSpace(p.Doc) == "" {
		if p.IsCommand {
			return Status{Problem: Missing, Suggest: "// " + commandName(p) + " ..."}
		}
		return Status{Problem: Missing, Suggest: "// Package " + p.Name + " ..."}
	}
	if p.IsCommand {
		return Status{}
	}
	if !strings.HasPrefix(p.Doc, "Package "+p.Name) {
		return Status{Problem: Style, Suggest: "// Package " + p.Name + " ..."}
	}
	return Status{}
}

// CheckValue checks a const or var declaration. It is documented if it has a
// comment above it (for a group, above "const (") or if every name in it has
// a comment of its own, above its line or at the end of it.
//
// A group with a comment above it but some names without their own comment is
// documented by Go's rules, so it only gets a tip listing those names.
func CheckValue(v *extract.Value) Status {
	if len(v.Undocumented) == 0 {
		return Status{}
	}
	if strings.TrimSpace(v.Doc) != "" {
		if len(v.Names) == 1 {
			return Status{}
		}
		return Status{Problem: Style, Group: true, Undocumented: v.Undocumented, GroupSize: len(v.Names)}
	}
	s := Status{Problem: Missing, Suggest: "// " + v.Names[0] + " ..."}
	if len(v.Names) > 1 {
		s.Group = true
		s.Undocumented = v.Undocumented
		s.GroupSize = len(v.Names)
		s.Suggest = "// Describe what these have in common."
	}
	return s
}

// Check checks the doc comment of a func, method or type named name.
// "A", "An" and "The" are allowed before the name, as in "A Reader reads...".
func Check(name, doc string) Status {
	doc = strings.TrimSpace(doc)
	if doc == "" {
		return Status{Problem: Missing, Suggest: "// " + name + " ..."}
	}
	word := firstWord(doc)
	switch word {
	case "A", "An", "The":
		word = firstWord(strings.TrimSpace(doc[len(word):]))
	}
	if word != name {
		return Status{Problem: Style, Suggest: "// " + name + " ..."}
	}
	return Status{}
}

func firstWord(s string) string {
	end := strings.IndexFunc(s, func(r rune) bool {
		return !(unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_')
	})
	if end < 0 {
		return s
	}
	return s[:end]
}

func commandName(p *extract.Package) string {
	name := p.RelDir
	if i := strings.LastIndexByte(name, '/'); i >= 0 {
		name = name[i+1:]
	}
	if name == "." || name == "" {
		return "Command"
	}
	return strings.ToUpper(name[:1]) + name[1:]
}

func hasHave(n int) string {
	if n == 1 {
		return "has"
	}
	return "have"
}

// OwnCommentDetail says which names in a group have no comment of their own,
// e.g. "TownX, TownZ have no comment of their own".
func OwnCommentDetail(names []string) string {
	if len(names) == 1 {
		return names[0] + " has no comment of its own"
	}
	return strings.Join(names, ", ") + " have no comment of their own"
}

func themIt(n int) string {
	if n == 1 {
		return "it"
	}
	return "them"
}
