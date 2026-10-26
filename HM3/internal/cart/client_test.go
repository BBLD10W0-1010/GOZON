package cart

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"gozon/internal/productpb"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/status"
)

type stubClient func(context.Context, *productpb.GetProductRequest) (*productpb.Product, error)

func (f stubClient) GetProduct(ctx context.Context, req *productpb.GetProductRequest, _ ...grpc.CallOption) (*productpb.Product, error) {
	return f(ctx, req)
}

func catalogClient() stubClient {
	return pricedClient(10)
}

func pricedClient(price int32) stubClient {
	return func(_ context.Context, req *productpb.GetProductRequest) (*productpb.Product, error) {
		return &productpb.Product{Sku: req.GetSku(), Name: "Book", Price: price}, nil
	}
}

func seedCart(t *testing.T, skus ...int64) *Service {
	t.Helper()
	s := NewService(catalogClient())
	for _, sku := range skus {
		require.NoError(t, s.AddToCart(t.Context(), 1, sku, 1))
	}
	return s
}

func parallelClient(size int32) (stubClient, <-chan time.Time) {
	var calls atomic.Int32
	gate := make(chan struct{})
	deadlines := make(chan time.Time, size)
	client := stubClient(func(ctx context.Context, req *productpb.GetProductRequest) (*productpb.Product, error) {
		deadline, _ := ctx.Deadline()
		deadlines <- deadline
		if calls.Add(1) == size {
			close(gate)
		}
		select {
		case <-gate:
			return &productpb.Product{Sku: req.GetSku(), Price: 10}, nil
		case <-ctx.Done():
			return nil, status.FromContextError(ctx.Err()).Err()
		}
	})
	return client, deadlines
}
