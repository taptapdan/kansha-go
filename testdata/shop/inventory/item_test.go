package inventory_test

import (
	"fmt"

	"example.com/shop/inventory"
)

func ExampleItem_Total() {
	it := inventory.NewItem("APL-001", "Apple", 45)
	it.Qty = 10
	fmt.Println(it.Total())
	// Output: 450
}

// Restock adds the same amount to every item.
func ExampleRestock() {
	a := inventory.NewItem("APL-001", "Apple", 45)
	b := inventory.NewItem("BRD-002", "Bread", 300)
	inventory.Restock([]*inventory.Item{a, b}, 3)
	fmt.Println(a.Qty, b.Qty)
	// Output: 3 3
}
