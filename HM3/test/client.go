//go:build api

package test

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"math"
	"math/big"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"gozon/internal/productpb"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type clients struct {
	products productpb.ProductServiceClient
	http     *http.Client
	cartURL  string
}

type cartItem struct {
	SKU   int64  `json:"sku"`
	Name  string `json:"name"`
	Count uint32 `json:"count"`
	Price uint32 `json:"price"`
}

type cartResponse struct {
	Items      []cartItem `json:"items"`
	TotalPrice uint32     `json:"total_price"`
}

func newClients(t *testing.T) clients {
	t.Helper()
	target := os.Getenv("PRODUCT_GRPC_TARGET")
	if target == "" {
		target = "localhost:50051"
	}
	url := os.Getenv("CART_HTTP_URL")
	if url == "" {
		url = "http://localhost:8080"
	}
	conn, err := grpc.NewClient(target, grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, conn.Close()) })
	transport := http.DefaultTransport.(*http.Transport).Clone()
	t.Cleanup(transport.CloseIdleConnections)
	return clients{
		products: productpb.NewProductServiceClient(conn),
		http:     &http.Client{Timeout: 4 * time.Second, Transport: transport},
		cartURL:  strings.TrimRight(url, "/"),
	}
}

func (c clients) request(t *testing.T, method, path, body string) *http.Response {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), method, c.cartURL+path, strings.NewReader(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	response, err := c.http.Do(req)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, response.Body.Close()) })
	return response
}

func (c clients) add(t *testing.T, path string, sku int64, count uint32) {
	t.Helper()
	response := c.request(t, http.MethodPost, fmt.Sprintf("%s/%d", path, sku), fmt.Sprintf(`{"count":%d}`, count))
	require.Equal(t, http.StatusOK, response.StatusCode)
}

func (c clients) cart(t *testing.T, path string) cartResponse {
	t.Helper()
	response := c.request(t, http.MethodGet, path, "")
	require.Equal(t, http.StatusOK, response.StatusCode)
	require.Equal(t, "application/json", response.Header.Get("Content-Type"))
	var cart cartResponse
	require.NoError(t, json.NewDecoder(response.Body).Decode(&cart))
	return cart
}

func (c clients) catalog(t *testing.T) []*productpb.Product {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
	defer cancel()
	var products []*productpb.Product
	page := int32(1)
	for page != 0 {
		response, err := c.products.ListProducts(ctx, &productpb.ListProductsRequest{Page: page, PageSize: 100})
		require.NoError(t, err)
		products = append(products, response.GetProducts()...)
		page = response.GetNextPage()
	}
	return products
}

func newUserID(t *testing.T) int64 {
	t.Helper()
	value, err := rand.Int(rand.Reader, big.NewInt(math.MaxInt64-1))
	require.NoError(t, err)
	return value.Int64() + 1
}

func missingSKU(products []*productpb.Product) int64 {
	exists := make(map[int64]bool, len(products))
	for _, product := range products {
		exists[product.GetSku()] = true
	}
	sku := int64(1)
	for exists[sku] {
		sku++
	}
	return sku
}
