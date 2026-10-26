package main

import (
	"context"
	"net"
	"testing"
	"time"

	"gozon/internal/product"
	"gozon/internal/productpb"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func TestRun(t *testing.T) {
	t.Parallel()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = listener.Close() })
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		done <- run(ctx, listener, map[int64]product.Item{10: {SKU: 10, Name: "Book", Price: 10}})
	}()
	conn, err := grpc.NewClient(listener.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	defer conn.Close()
	requestCtx, requestCancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer requestCancel()
	got, err := productpb.NewProductServiceClient(conn).GetProduct(requestCtx, &productpb.GetProductRequest{Sku: 10})
	require.NoError(t, err)
	require.Equal(t, "Book", got.GetName())
	require.Equal(t, int32(10), got.GetPrice())
	cancel()
	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(3 * time.Second):
		t.Fatal("gRPC server did not stop")
	}
}

func TestRunClosedListener(t *testing.T) {
	t.Parallel()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	require.NoError(t, listener.Close())
	require.Error(t, run(t.Context(), listener, nil))
}
