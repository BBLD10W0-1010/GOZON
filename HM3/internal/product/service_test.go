package product

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"gozon/internal/productpb"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func testCatalog() map[int64]Item {
	return map[int64]Item{
		20: {SKU: 20, Name: "Second", Price: 42},
		10: {SKU: 10, Name: "First", Price: 7},
	}
}

func TestGetProduct(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		req  *productpb.GetProductRequest
		want *productpb.Product
		code codes.Code
	}{
		{"existing", &productpb.GetProductRequest{Sku: 10}, &productpb.Product{Sku: 10, Name: "First", Price: 7}, codes.OK},
		{"missing", &productpb.GetProductRequest{Sku: 99}, nil, codes.NotFound},
		{"zero", &productpb.GetProductRequest{}, nil, codes.InvalidArgument},
		{"negative", &productpb.GetProductRequest{Sku: -1}, nil, codes.InvalidArgument},
		{"nil request", nil, nil, codes.InvalidArgument},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			s := NewServer(testCatalog())
			got, err := s.GetProduct(context.Background(), tc.req)
			require.Equal(t, tc.code, status.Code(err))
			require.Equal(t, tc.want, got)
		})
	}
}

func TestListProducts(t *testing.T) {
	t.Parallel()
	first := &productpb.Product{Sku: 10, Name: "First", Price: 7}
	second := &productpb.Product{Sku: 20, Name: "Second", Price: 42}
	for _, tc := range []struct {
		name string
		req  *productpb.ListProductsRequest
		want *productpb.ListProductsResponse
		code codes.Code
	}{
		{"defaults", &productpb.ListProductsRequest{}, &productpb.ListProductsResponse{Products: []*productpb.Product{first, second}, Total: 2}, codes.OK},
		{"nil request", nil, &productpb.ListProductsResponse{Products: []*productpb.Product{first, second}, Total: 2}, codes.OK},
		{"first page", &productpb.ListProductsRequest{Page: 1, PageSize: 1}, &productpb.ListProductsResponse{Products: []*productpb.Product{first}, Total: 2, NextPage: 2}, codes.OK},
		{"last page", &productpb.ListProductsRequest{Page: 2, PageSize: 1}, &productpb.ListProductsResponse{Products: []*productpb.Product{second}, Total: 2}, codes.OK},
		{"past end", &productpb.ListProductsRequest{Page: 3, PageSize: 1}, &productpb.ListProductsResponse{Total: 2}, codes.OK},
		{"negative page", &productpb.ListProductsRequest{Page: -1}, nil, codes.InvalidArgument},
		{"negative size", &productpb.ListProductsRequest{PageSize: -1}, nil, codes.InvalidArgument},
		{"large size", &productpb.ListProductsRequest{PageSize: 101}, nil, codes.InvalidArgument},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			s := NewServer(testCatalog())
			got, err := s.ListProducts(context.Background(), tc.req)
			require.Equal(t, tc.code, status.Code(err))
			require.Equal(t, tc.want, got)
		})
	}
}

func TestEmptyCatalog(t *testing.T) {
	t.Parallel()
	s := NewServer(nil)
	got, err := s.ListProducts(context.Background(), &productpb.ListProductsRequest{})
	require.NoError(t, err)
	require.Empty(t, got.GetProducts())
	require.Zero(t, got.GetTotal())
}

func TestCatalogIsolation(t *testing.T) {
	t.Parallel()
	items := testCatalog()
	s := NewServer(items)
	delete(items, 10)
	got, err := s.GetProduct(context.Background(), &productpb.GetProductRequest{Sku: 10})
	require.NoError(t, err)
	got.Name = "Changed"
	again, err := s.GetProduct(context.Background(), &productpb.GetProductRequest{Sku: 10})
	require.NoError(t, err)
	require.Equal(t, "First", again.GetName())
}

func TestLoad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		json string
		want map[int64]Item
	}{
		{"catalog", `{"20":{"name":"Second","price":42},"10":{"name":"First","price":7}}`, testCatalog()},
		{"empty", `{}`, map[int64]Item{}},
		{"zero price", `{"1":{"name":"Free","price":0}}`, map[int64]Item{1: {SKU: 1, Name: "Free", Price: 0}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			path := filepath.Join(t.TempDir(), "products.json")
			require.NoError(t, os.WriteFile(path, []byte(tc.json), 0600))
			got, err := Load(path)
			require.NoError(t, err)
			require.Equal(t, tc.want, got)
		})
	}
}

func TestLoadInvalidCatalog(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ name, json string }{
		{"null", `null`}, {"malformed", `{`}, {"array", `[]`},
		{"invalid SKU", `{"abc":{"name":"Bad","price":1}}`},
		{"zero SKU", `{"0":{"name":"Bad","price":1}}`},
		{"negative SKU", `{"-1":{"name":"Bad","price":1}}`},
		{"empty name", `{"1":{"name":"","price":1}}`},
		{"blank name", `{"1":{"name":"  ","price":1}}`},
		{"missing price", `{"1":{"name":"Book"}}`},
		{"null price", `{"1":{"name":"Book","price":null}}`},
		{"duplicate numeric SKU", `{"1":{"name":"First","price":1},"01":{"name":"Second","price":2}}`},
		{"negative price", `{"1":{"name":"Bad","price":-1}}`},
		{"large price", `{"1":{"name":"Bad","price":2147483648}}`},
		{"fractional price", `{"1":{"name":"Bad","price":1.5}}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			path := filepath.Join(t.TempDir(), "products.json")
			require.NoError(t, os.WriteFile(path, []byte(tc.json), 0600))
			got, err := Load(path)
			require.Error(t, err)
			require.Nil(t, got)
		})
	}
}

func TestLoadMissingFile(t *testing.T) {
	t.Parallel()
	_, err := Load(filepath.Join(t.TempDir(), "missing.json"))
	require.ErrorIs(t, err, os.ErrNotExist)
}
