package inventory

func Restock(items []*Item, n int) {
	for _, it := range items {
		it.Qty += n
	}
}
