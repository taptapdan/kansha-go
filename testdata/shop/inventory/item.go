package inventory

import "fmt"

// LowStock is the quantity at or below which an item should be reordered.
const LowStock = 5

// Category groups items for reporting.
type Category int

// The categories an item can belong to.
const (
	Produce Category = iota // Fresh fruit and vegetables.
	Bakery                  // Bread and pastries.
	Pantry
)

// Item is a single product on the shelf. The zero value is not useful; create
// items with [NewItem].
type Item struct {
	SKU        string // Stock keeping unit, e.g. "APL-001".
	Name       string
	PriceCents int // Unit price in cents.
	Qty        int // Units on the shelf right now.
	Category   Category

	notes []string
}

// NewItem returns an Item with zero quantity. It panics if priceCents is
// negative.
func NewItem(sku, name string, priceCents int) *Item {
	if priceCents < 0 {
		panic("inventory: negative price")
	}
	return &Item{SKU: sku, Name: name, PriceCents: priceCents}
}

// Total returns the value of everything on the shelf, in cents.
func (i *Item) Total() int {
	return i.PriceCents * i.Qty
}

// returns true when the item is at or below [LowStock].
func (i *Item) NeedsReorder() bool {
	return i.Qty <= LowStock
}

// Pricer is anything that can report a price in cents.
type Pricer interface {
	// Price returns the unit price in cents.
	Price() int
	fmt.Stringer
}

// Limits documented one name at a time, with no comment above the group.
// (This comment is separated by a blank line, so it doesn't count.)

const (
	// MaxQty is the most units one shelf holds.
	MaxQty   = 500
	MinPrice = 1 // Lowest allowed price in cents.
)

var (
	// DefaultCategory is used when none is given.
	DefaultCategory = Pantry
	Currency        = "USD"
	Locale          = "en-US"
)
