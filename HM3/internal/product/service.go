package product

import (
	"context"
	"sort"

	"gozon/internal/productpb"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Item struct {
	SKU   int64
	Name  string
	Price uint32
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
	page, size, err := pagination(req)
	if err != nil {
		return nil, err
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

func pagination(req *productpb.ListProductsRequest) (int32, int32, error) {
	page, size := req.GetPage(), req.GetPageSize()
	if page < 0 || size < 0 || size > 100 {
		return 0, 0, status.Error(codes.InvalidArgument, "page must not be negative; page_size must be between 0 and 100")
	}
	if page == 0 {
		page = 1
	}
	if size == 0 {
		size = 20
	}
	return page, size, nil
}
