package product

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"gozon/internal/productpb"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestLoadAndProductRPC(t *testing.T) {
	path := filepath.Join(t.TempDir(), "products.json")
	if err := os.WriteFile(path, []byte(`{"20":{"name":"Second","price":42},"10":{"name":"First","price":7}}`), 0600); err != nil {
		t.Fatal(err)
	}
	items, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	s := NewServer(items)
	for _, tc := range []struct {
		sku  int64
		code codes.Code
	}{
		{0, codes.InvalidArgument}, {-1, codes.InvalidArgument}, {99, codes.NotFound},
	} {
		_, err := s.GetProduct(context.Background(), &productpb.GetProductRequest{Sku: tc.sku})
		if status.Code(err) != tc.code {
			t.Errorf("GetProduct(%d) code = %v, want %v", tc.sku, status.Code(err), tc.code)
		}
	}
	got, err := s.GetProduct(context.Background(), &productpb.GetProductRequest{Sku: 10})
	if err != nil || got.GetName() != "First" || got.GetPrice() != 7 || got.GetSku() != 10 {
		t.Fatalf("GetProduct(10) = %v, %v", got, err)
	}
	page, err := s.ListProducts(context.Background(), &productpb.ListProductsRequest{Page: 1, PageSize: 1})
	if err != nil || page.GetTotal() != 2 || page.GetNextPage() != 2 || len(page.GetProducts()) != 1 || page.GetProducts()[0].GetSku() != 10 {
		t.Fatalf("first page = %v, %v", page, err)
	}
	page, err = s.ListProducts(context.Background(), &productpb.ListProductsRequest{Page: 2, PageSize: 1})
	if err != nil || page.GetNextPage() != 0 || len(page.GetProducts()) != 1 || page.GetProducts()[0].GetSku() != 20 {
		t.Fatalf("second page = %v, %v", page, err)
	}
	_, err = s.ListProducts(context.Background(), &productpb.ListProductsRequest{PageSize: 101})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("invalid page_size code = %v", status.Code(err))
	}
}

func TestLoadRejectsInvalidCatalog(t *testing.T) {
	for _, contents := range []string{
		`null`, `{"0":{"name":"Bad","price":1}}`,
		`{"1":{"name":"Bad","price":-1}}`,
		`{"1":{"name":"Bad","price":2147483648}}`,
	} {
		path := filepath.Join(t.TempDir(), "products.json")
		if err := os.WriteFile(path, []byte(contents), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := Load(path); err == nil {
			t.Errorf("Load(%s) unexpectedly succeeded", contents)
		}
	}
}
