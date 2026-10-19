package coverage

import "testing"

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
