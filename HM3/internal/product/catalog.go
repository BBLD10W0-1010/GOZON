package product

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
)

type catalogRecord struct {
	Name  string `json:"name"`
	Price *int64 `json:"price"`
}

func Load(path string) (map[int64]Item, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read products: %w", err)
	}
	var records map[string]catalogRecord
	if err := json.Unmarshal(data, &records); err != nil {
		return nil, fmt.Errorf("decode products: %w", err)
	}
	if records == nil {
		return nil, fmt.Errorf("products must be a JSON object")
	}
	return catalogItems(records)
}

func catalogItems(records map[string]catalogRecord) (map[int64]Item, error) {
	items := make(map[int64]Item, len(records))
	for key, record := range records {
		item, err := record.item(key)
		if err != nil {
			return nil, err
		}
		if _, exists := items[item.SKU]; exists {
			return nil, fmt.Errorf("duplicate product SKU %q", key)
		}
		items[item.SKU] = item
	}
	return items, nil
}

func (r catalogRecord) item(key string) (Item, error) {
	sku, err := strconv.ParseInt(key, 10, 64)
	if err != nil || sku <= 0 {
		return Item{}, fmt.Errorf("invalid product SKU %q", key)
	}
	if strings.TrimSpace(r.Name) == "" || r.Price == nil || *r.Price < 0 || *r.Price > math.MaxInt32 {
		return Item{}, fmt.Errorf("invalid product %q: name and int32 price are required", key)
	}
	return Item{SKU: sku, Name: r.Name, Price: uint32(*r.Price)}, nil
}
