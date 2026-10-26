package cart

import (
	"math"

	"gozon/internal/productpb"
)

type Item struct {
	SKU   int64  `json:"sku"`
	Name  string `json:"name"`
	Count uint32 `json:"count"`
	Price uint32 `json:"price"`
}

func (i Item) subtotal() uint64 {
	return uint64(i.Price) * uint64(i.Count)
}

func productItem(p *productpb.Product, count uint32) Item {
	return Item{SKU: p.GetSku(), Name: p.GetName(), Count: count, Price: uint32(p.GetPrice())}
}

type Cart struct {
	Items      []Item `json:"items"`
	TotalPrice uint32 `json:"total_price"`
}

func makeCart(items []Item) (Cart, error) {
	var total uint64
	for _, item := range items {
		total += item.subtotal()
		if total > math.MaxUint32 {
			return Cart{}, ErrOverflow
		}
	}
	return Cart{Items: items, TotalPrice: uint32(total)}, nil
}

type cartData struct {
	items map[int64]Item
	total uint32
}

func (c *cartData) add(item Item) error {
	previous := c.items[item.SKU]
	count := uint64(previous.Count) + uint64(item.Count)
	if count > math.MaxUint32 {
		return ErrOverflow
	}
	item.Count = uint32(count)
	total := uint64(c.total) - previous.subtotal() + item.subtotal()
	if total > math.MaxUint32 {
		return ErrOverflow
	}
	c.items[item.SKU] = item
	c.total = uint32(total)
	return nil
}
