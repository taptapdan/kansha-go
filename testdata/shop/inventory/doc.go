// Package inventory tracks items in a small shop: what is in stock, what it
// costs, and when to reorder. Prices are stored in cents to avoid floating
// point rounding.
//
// Create items with [NewItem] and adjust their quantity with [Restock]. For
// formatting prices, see [fmt.Sprintf].
//
// # Usage
//
// A typical flow creates an item, sells a few, and checks whether it should be
// reordered:
//
//	apple := inventory.NewItem("APL-001", "Apple", 45)
//	apple.Qty = 12
//	if apple.NeedsReorder() {
//		log.Println("order more apples")
//	}
//
// # Rules
//
//   - Quantities never go below zero.
//   - A [Category] groups items for reporting.
package inventory
