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
	if len(inv.Bugs) != 1 {
		t.Errorf("bugs = %v", inv.Bugs)
	}
}
