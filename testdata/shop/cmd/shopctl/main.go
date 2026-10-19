// Shopctl prints a quick inventory report.
package main

import (
	"fmt"

	"example.com/shop/inventory"
)

// main builds a couple of items and prints their total value.
func main() {
	apple := inventory.NewItem("APL-001", "Apple", 45)
	apple.Qty = 12
	fmt.Println(report(apple))
}

func report(it *inventory.Item) string {
	return fmt.Sprintf("%s: %d cents", it.Name, it.Total())
}
