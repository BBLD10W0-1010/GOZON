package cart

import (
	"context"
	"testing"

	"gozon/internal/productpb"
)

func BenchmarkAddToCart(b *testing.B) {
	s := NewService(stubClient(func(_ context.Context, req *productpb.GetProductRequest) (*productpb.Product, error) {
		return &productpb.Product{Sku: req.GetSku()}, nil
	}))
	ctx := context.Background()
	if err := s.AddToCart(ctx, 1, 10, 1); err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		if err := s.AddToCart(ctx, 1, 10, 1); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkGetCart(b *testing.B) {
	s := NewService(catalogClient())
	ctx := context.Background()
	for _, sku := range []int64{10, 20, 30} {
		if err := s.AddToCart(ctx, 1, sku, 1); err != nil {
			b.Fatal(err)
		}
	}
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		if _, err := s.GetCart(ctx, 1); err != nil {
			b.Fatal(err)
		}
	}
}
