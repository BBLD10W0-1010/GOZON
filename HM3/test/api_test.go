//go:build api

package test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"gozon/internal/productpb"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestCartWorkflow(t *testing.T) {
	t.Parallel()
	c := newClients(t)
	products := c.catalog(t)
	require.GreaterOrEqual(t, len(products), 2, "the catalog must contain at least two products")
	first, second := products[0], products[1]
	require.Less(t, first.GetSku(), second.GetSku())
	userID := newUserID(t)
	cartPath := fmt.Sprintf("/user/%d/cart", userID)
	require.Equal(t, http.StatusNotFound, c.request(t, http.MethodGet, cartPath, "").StatusCode)
	for _, line := range []struct {
		sku   int64
		count uint32
	}{{second.GetSku(), 2}, {first.GetSku(), 1}, {first.GetSku(), 2}} {
		response := c.request(t, http.MethodPost, fmt.Sprintf("%s/%d", cartPath, line.sku), fmt.Sprintf(`{"count":%d}`, line.count))
		require.Equal(t, http.StatusOK, response.StatusCode)
	}
	response := c.request(t, http.MethodGet, cartPath, "")
	require.Equal(t, http.StatusOK, response.StatusCode)
	require.Equal(t, "application/json", response.Header.Get("Content-Type"))
	type item struct {
		SKU   int64  `json:"sku"`
		Name  string `json:"name"`
		Count uint32 `json:"count"`
		Price uint32 `json:"price"`
	}
	var got struct {
		Items      []item `json:"items"`
		TotalPrice uint32 `json:"total_price"`
	}
	require.NoError(t, json.NewDecoder(response.Body).Decode(&got))
	require.Equal(t, []item{
		{SKU: first.GetSku(), Name: first.GetName(), Count: 3, Price: uint32(first.GetPrice())},
		{SKU: second.GetSku(), Name: second.GetName(), Count: 2, Price: uint32(second.GetPrice())},
	}, got.Items)
	require.Equal(t, uint32(first.GetPrice())*3+uint32(second.GetPrice())*2, got.TotalPrice)
}

func TestHTTPErrorCodes(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, method, path, body string
		want                     int
	}{
		{"empty cart", http.MethodGet, "/user/{user}/cart", "", http.StatusNotFound},
		{"zero user GET", http.MethodGet, "/user/0/cart", "", http.StatusBadRequest},
		{"negative user GET", http.MethodGet, "/user/-1/cart", "", http.StatusBadRequest},
		{"zero user POST", http.MethodPost, "/user/0/cart/1", `{"count":1}`, http.StatusBadRequest},
		{"negative user POST", http.MethodPost, "/user/-1/cart/1", `{"count":1}`, http.StatusBadRequest},
		{"zero SKU", http.MethodPost, "/user/{user}/cart/0", `{"count":1}`, http.StatusBadRequest},
		{"negative SKU", http.MethodPost, "/user/{user}/cart/-1", `{"count":1}`, http.StatusBadRequest},
		{"zero count", http.MethodPost, "/user/{user}/cart/1", `{"count":0}`, http.StatusBadRequest},
		{"negative count", http.MethodPost, "/user/{user}/cart/1", `{"count":-1}`, http.StatusBadRequest},
		{"missing product", http.MethodPost, "/user/{user}/cart/{missing}", `{"count":1}`, http.StatusPreconditionFailed},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			c := newClients(t)
			path := strings.ReplaceAll(tc.path, "{user}", strconv.FormatInt(newUserID(t), 10))
			path = strings.ReplaceAll(path, "{missing}", strconv.FormatInt(missingSKU(c.catalog(t)), 10))
			response := c.request(t, tc.method, path, tc.body)
			require.Equal(t, tc.want, response.StatusCode)
		})
	}
}

func TestProductErrorCodes(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		sku  int64
		code codes.Code
	}{
		{"zero", 0, codes.InvalidArgument},
		{"negative", -1, codes.InvalidArgument},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			c := newClients(t)
			ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
			defer cancel()
			_, err := c.products.GetProduct(ctx, &productpb.GetProductRequest{Sku: tc.sku})
			require.Equal(t, tc.code, status.Code(err))
		})
	}
}

func TestProductLookup(t *testing.T) {
	t.Parallel()
	c := newClients(t)
	catalog := c.catalog(t)
	require.NotEmpty(t, catalog)
	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
	defer cancel()
	got, err := c.products.GetProduct(ctx, &productpb.GetProductRequest{Sku: catalog[0].GetSku()})
	require.NoError(t, err)
	require.Equal(t, catalog[0].GetSku(), got.GetSku())
	require.Equal(t, catalog[0].GetName(), got.GetName())
	require.Equal(t, catalog[0].GetPrice(), got.GetPrice())
	_, err = c.products.GetProduct(ctx, &productpb.GetProductRequest{Sku: missingSKU(catalog)})
	require.Equal(t, codes.NotFound, status.Code(err))
}
