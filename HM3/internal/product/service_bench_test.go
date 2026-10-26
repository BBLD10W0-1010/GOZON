package product

import (
	"context"
	"testing"

	"gozon/internal/productpb"
)

func BenchmarkGetProduct(b *testing.B) {
	s := NewServer(testCatalog())
	req := &productpb.GetProductRequest{Sku: 10}
	ctx := context.Background()
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		if _, err := s.GetProduct(ctx, req); err != nil {
			b.Fatal(err)
		}
	}
}
