package product

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"sort"
	"strconv"

	"gozon/internal/productpb"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Item struct {
	SKU   int64
	Name  string
	Price uint32
}

// Load reads and validates the entire catalog before the server starts.
func Load(path string) (map[int64]Item, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read products: %w", err)
	}
	var records map[string]struct {
		Name  string `json:"name"`
		Price int64  `json:"price"`
	}
	if err := json.Unmarshal(data, &records); err != nil {
		return nil, fmt.Errorf("decode products: %w", err)
	}
	if records == nil {
		return nil, fmt.Errorf("products must be a JSON object")
	}
	items := make(map[int64]Item, len(records))
	for key, record := range records {
		sku, err := strconv.ParseInt(key, 10, 64)
		if err != nil || sku <= 0 {
			return nil, fmt.Errorf("invalid product SKU %q", key)
		}
		if record.Name == "" || record.Price < 0 || record.Price > math.MaxInt32 {
			return nil, fmt.Errorf("invalid product %q: name is required and price must fit int32", key)
		}
		items[sku] = Item{SKU: sku, Name: record.Name, Price: uint32(record.Price)}
	}
	return items, nil
}

type Server struct {
	productpb.UnimplementedProductServiceServer
	items map[int64]Item
	skus  []int64
}

func NewServer(items map[int64]Item) *Server {
	copyItems := make(map[int64]Item, len(items))
	skus := make([]int64, 0, len(items))
	for sku, item := range items {
		copyItems[sku] = item
		skus = append(skus, sku)
	}
	sort.Slice(skus, func(i, j int) bool { return skus[i] < skus[j] })
	return &Server{items: copyItems, skus: skus}
}

func toProto(item Item) *productpb.Product {
	return &productpb.Product{Sku: item.SKU, Name: item.Name, Price: int32(item.Price)}
}

func (s *Server) GetProduct(_ context.Context, req *productpb.GetProductRequest) (*productpb.Product, error) {
	if req == nil || req.GetSku() <= 0 {
		return nil, status.Error(codes.InvalidArgument, "sku must be a positive integer")
	}
	item, ok := s.items[req.GetSku()]
	if !ok {
		return nil, status.Error(codes.NotFound, "product not found")
	}
	return toProto(item), nil
}

func (s *Server) ListProducts(_ context.Context, req *productpb.ListProductsRequest) (*productpb.ListProductsResponse, error) {
	if req == nil {
		req = &productpb.ListProductsRequest{}
	}
	page, size := req.GetPage(), req.GetPageSize()
	if page < 0 || size < 0 || size > 100 {
		return nil, status.Error(codes.InvalidArgument, "page and page_size must be positive; page_size must not exceed 100")
	}
	if page == 0 {
		page = 1
	}
	if size == 0 {
		size = 20
	}
	start := int64(page-1) * int64(size)
	response := &productpb.ListProductsResponse{Total: int64(len(s.skus))}
	if start >= int64(len(s.skus)) {
		return response, nil
	}
	end := min(start+int64(size), int64(len(s.skus)))
	for _, sku := range s.skus[int(start):int(end)] {
		response.Products = append(response.Products, toProto(s.items[sku]))
	}
	if end < int64(len(s.skus)) {
		response.NextPage = page + 1
	}
	return response, nil
}
