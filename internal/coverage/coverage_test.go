package coverage

import (
	"testing"

	"github.com/taptapdan/kansha-go/internal/extract"
)

func TestCheck(t *testing.T) {
	tests := []struct {
		name, doc, want string
	}{
		{"Restock", "", Missing},
		{"Restock", "Restock adds n units.", ""},
		{"Reader", "A Reader reads bytes.", ""},
		{"NeedsReorder", "returns true when low.", Style},
		{"Item", "Item, the thing on the shelf.", ""},
	}
	for _, tt := range tests {
		if got := Check(tt.name, tt.doc).Problem; got != tt.want {
			t.Errorf("Check(%q, %q) = %q, want %q", tt.name, tt.doc, got, tt.want)
		}
	}
}

func TestCheckValue(t *testing.T) {
	tests := []struct {
		desc    string
		v       extract.Value
		problem string
		group   bool
	}{
		{"comment above the group", extract.Value{Names: []string{"TownX", "TownZ"}, Doc: "The starting town.", Undocumented: []string{"TownX", "TownZ"}}, "", false},
		{"every name has its own comment", extract.Value{Names: []string{"WorldSize", "CellSize"}}, "", false},
		{"some names missing", extract.Value{Names: []string{"A", "B", "C"}, Undocumented: []string{"B", "C"}}, Missing, true},
		{"single const without comment", extract.Value{Names: []string{"LowStock"}, Undocumented: []string{"LowStock"}}, Missing, false},
	}
	for _, tt := range tests {
		s := CheckValue(&tt.v)
		if s.Problem != tt.problem || s.Group != tt.group {
			t.Errorf("%s: got problem %q group %v, want %q group %v", tt.desc, s.Problem, s.Group, tt.problem, tt.group)
		}
	}
}
