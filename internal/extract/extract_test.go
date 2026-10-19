package extract

import (
	"strings"
	"testing"
)

func TestLoadShop(t *testing.T) {
	m, err := Load(Options{Dir: "../../testdata/shop"})
	if err != nil {
		t.Fatal(err)
	}
	if m.Path != "example.com/shop" {
		t.Errorf("module path = %q", m.Path)
	}
	if len(m.Packages) != 2 {
		t.Fatalf("got %d packages, want 2", len(m.Packages))
	}

	cmd, inv := m.Packages[0], m.Packages[1]
	if !cmd.IsCommand || cmd.RelDir != "cmd/shopctl" {
		t.Errorf("first package = %s (command %v)", cmd.RelDir, cmd.IsCommand)
	}
	if len(cmd.Funcs) != 2 {
		t.Errorf("command should include unexported funcs, got %d", len(cmd.Funcs))
	}

	if !strings.HasPrefix(inv.Doc, "Package inventory tracks") {
		t.Errorf("package doc = %q", inv.Doc)
	}
	if inv.DocPos.File != "inventory/doc.go" {
		t.Errorf("doc pos = %+v", inv.DocPos)
	}

	var item *Type
	for _, ty := range inv.Types {
		if ty.Name == "Item" {
			item = ty
		}
	}
	if item == nil {
		t.Fatal("type Item not found")
	}
	if item.Kind != "struct" || !item.Hidden {
		t.Errorf("Item kind=%s hidden=%v", item.Kind, item.Hidden)
	}
	if len(item.Fields) != 5 || item.Fields[0].Doc != `Stock keeping unit, e.g. "APL-001".` {
		t.Errorf("fields = %+v", item.Fields[0])
	}
	if !strings.Contains(item.Code, "// Unit price in cents.") || strings.Contains(item.Code, "Item is a single") {
		t.Errorf("type code:\n%s", item.Code)
	}
	if len(item.Funcs) != 1 || item.Funcs[0].Name != "NewItem" {
		t.Errorf("constructors = %v", item.Funcs)
	}
	if len(item.Methods) != 2 {
		t.Errorf("methods = %d", len(item.Methods))
	}
	total := item.Methods[1]
	if total.Signature != "func (i *Item) Total() int" {
		t.Errorf("signature = %q", total.Signature)
	}
	if len(total.Examples) != 1 || total.Examples[0].Output != "450\n" || strings.Contains(total.Examples[0].Code, "Output") {
		t.Errorf("example = %+v", total.Examples)
	}
	undocumented := map[string][]string{}
	for _, v := range append(inv.Consts, inv.Vars...) {
		undocumented[v.Names[0]] = v.Undocumented
	}
	if got := undocumented["MaxQty"]; len(got) != 0 {
		t.Errorf("MaxQty group: undocumented = %v, want none (line and trailing comments count)", got)
	}
	if got := strings.Join(undocumented["DefaultCategory"], ","); got != "Currency,Locale" {
		t.Errorf("DefaultCategory group: undocumented = %q, want Currency,Locale", got)
	}

	if len(inv.Bugs) != 1 {
		t.Errorf("bugs = %v", inv.Bugs)
	}
}

func findType(p *Package, name string) *Type {
	for _, ty := range p.Types {
		if ty.Name == name {
			return ty
		}
	}
	return nil
}

func TestHiddenFieldsNote(t *testing.T) {
	m, err := Load(Options{Dir: "../../testdata/shop"})
	if err != nil {
		t.Fatal(err)
	}
	inv := m.Packages[1]
	for _, name := range []string{"Item", "Shelf"} {
		ty := findType(inv, name)
		if n := strings.Count(ty.Code, filteredNote); n != 1 {
			t.Errorf("%s: note appears %d times, want 1:\n%s", name, n, ty.Code)
		}
		if strings.Contains(ty.Code, "\n\n\t"+filteredNote) {
			t.Errorf("%s: blank line before the note:\n%s", name, ty.Code)
		}
	}
}

func TestShowUnexported(t *testing.T) {
	m, err := Load(Options{Dir: "../../testdata/shop", Unexported: true})
	if err != nil {
		t.Fatal(err)
	}
	shelf := findType(m.Packages[1], "Shelf")
	if shelf.Hidden || strings.Contains(shelf.Code, filteredNote) {
		t.Errorf("unexported fields still hidden:\n%s", shelf.Code)
	}
	var names []string
	for _, f := range shelf.Fields {
		names = append(names, f.Name)
	}
	if got := strings.Join(names, ","); got != "id,Label,items" {
		t.Errorf("fields = %s, want id,Label,items", got)
	}
}
