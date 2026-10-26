package main

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"gozon/internal/productpb"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
)

type productStub struct{}

func (productStub) GetProduct(_ context.Context, req *productpb.GetProductRequest, _ ...grpc.CallOption) (*productpb.Product, error) {
	return &productpb.Product{Sku: req.GetSku(), Name: "Book", Price: 10}, nil
}

func TestRun(t *testing.T) {
	t.Parallel()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = listener.Close() })
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- run(ctx, listener, productStub{}) }()
	client := &http.Client{Timeout: 2 * time.Second}
	baseURL := "http://" + listener.Addr().String()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, baseURL+"/user/1/cart/10", strings.NewReader(`{"count":2}`))
	require.NoError(t, err)
	response, err := client.Do(req)
	require.NoError(t, err)
	require.NoError(t, response.Body.Close())
	require.Equal(t, http.StatusOK, response.StatusCode)
	response, err = client.Get(baseURL + "/user/1/cart")
	require.NoError(t, err)
	defer response.Body.Close()
	require.Equal(t, http.StatusOK, response.StatusCode)
	var got struct {
		TotalPrice uint32 `json:"total_price"`
	}
	require.NoError(t, json.NewDecoder(response.Body).Decode(&got))
	require.Equal(t, uint32(20), got.TotalPrice)
	cancel()
	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(6 * time.Second):
		t.Fatal("HTTP server did not stop")
	}
}

func TestRunClosedListener(t *testing.T) {
	t.Parallel()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	require.NoError(t, listener.Close())
	require.Error(t, run(t.Context(), listener, productStub{}))
}
