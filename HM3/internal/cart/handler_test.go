package cart

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"gozon/internal/productpb"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestHTTPAddValidation(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ name, path, body string }{
		{"zero user", "/user/0/cart/10", `{"count":1}`},
		{"negative user", "/user/-1/cart/10", `{"count":1}`},
		{"invalid user", "/user/abc/cart/10", `{"count":1}`},
		{"zero SKU", "/user/1/cart/0", `{"count":1}`},
		{"negative SKU", "/user/1/cart/-10", `{"count":1}`},
		{"invalid SKU", "/user/1/cart/abc", `{"count":1}`},
		{"zero count", "/user/1/cart/10", `{"count":0}`},
		{"negative count", "/user/1/cart/10", `{"count":-1}`},
		{"overflow", "/user/1/cart/10", `{"count":4294967296}`},
		{"fraction", "/user/1/cart/10", `{"count":1.5}`},
		{"string", "/user/1/cart/10", `{"count":"2"}`},
		{"null", "/user/1/cart/10", `null`},
		{"missing count", "/user/1/cart/10", `{}`},
		{"malformed", "/user/1/cart/10", `{`},
		{"empty", "/user/1/cart/10", ``},
		{"extra field", "/user/1/cart/10", `{"count":1,"extra":true}`},
		{"trailing JSON", "/user/1/cart/10", `{"count":1} {}`},
		{"large body", "/user/1/cart/10", strings.Repeat("1", 1025)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			h := NewHandler(catalogClient())
			w := httptest.NewRecorder()
			h.ServeHTTP(w, httptest.NewRequest(http.MethodPost, tc.path, strings.NewReader(tc.body)))
			require.Equal(t, http.StatusBadRequest, w.Code)
		})
	}
}

func TestHTTPProductErrors(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		code codes.Code
		want int
	}{
		{"not found", codes.NotFound, http.StatusPreconditionFailed},
		{"unavailable", codes.Unavailable, http.StatusInternalServerError},
		{"internal", codes.Internal, http.StatusInternalServerError},
		{"timeout", codes.DeadlineExceeded, http.StatusInternalServerError},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var failed atomic.Bool
			s := NewService(stubClient(func(_ context.Context, req *productpb.GetProductRequest) (*productpb.Product, error) {
				if failed.Load() {
					return nil, status.Error(tc.code, "catalog error")
				}
				return &productpb.Product{Sku: req.GetSku(), Price: 10}, nil
			}))
			require.NoError(t, s.AddToCart(context.Background(), 1, 10, 1))
			require.NoError(t, s.AddToCart(context.Background(), 1, 20, 1))
			failed.Store(true)
			h := newHandler(s)
			post := httptest.NewRecorder()
			h.ServeHTTP(post, httptest.NewRequest(http.MethodPost, "/user/1/cart/10", strings.NewReader(`{"count":1}`)))
			require.Equal(t, tc.want, post.Code)
			get := httptest.NewRecorder()
			h.ServeHTTP(get, httptest.NewRequest(http.MethodGet, "/user/1/cart", nil))
			require.Equal(t, tc.want, get.Code)
			require.NotContains(t, get.Body.String(), `"items"`)
		})
	}
}

func TestHTTPGetContract(t *testing.T) {
	t.Parallel()
	h := NewHandler(catalogClient())
	for _, path := range []string{"/user/1/cart/20", "/user/1/cart/10", "/user/1/cart/10"} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{"count":2}`)))
		require.Equal(t, http.StatusOK, w.Code)
		require.Empty(t, w.Body.String())
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/user/1/cart", nil))
	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, "application/json", w.Header().Get("Content-Type"))
	var got Cart
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	require.Equal(t, Cart{Items: []Item{
		{SKU: 10, Name: "Book", Price: 10, Count: 4},
		{SKU: 20, Name: "Book", Price: 10, Count: 2},
	}, TotalPrice: 60}, got)
	decoder := json.NewDecoder(bytes.NewReader(w.Body.Bytes()))
	decoder.UseNumber()
	var fields map[string]any
	require.NoError(t, decoder.Decode(&fields))
	require.IsType(t, json.Number(""), fields["total_price"])
	items := fields["items"].([]any)
	for _, value := range items {
		item := value.(map[string]any)
		require.IsType(t, json.Number(""), item["sku"])
		require.IsType(t, json.Number(""), item["price"])
		require.IsType(t, json.Number(""), item["count"])
	}
}

func TestHTTPGetErrors(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		path string
		want int
	}{
		{"/user/1/cart", http.StatusNotFound}, {"/user/0/cart", http.StatusBadRequest},
		{"/user/-1/cart", http.StatusBadRequest}, {"/user/abc/cart", http.StatusBadRequest},
	} {
		t.Run(tc.path, func(t *testing.T) {
			t.Parallel()
			h := NewHandler(catalogClient())
			w := httptest.NewRecorder()
			h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, tc.path, nil))
			require.Equal(t, tc.want, w.Code)
		})
	}
}

func TestHTTPNumericBody(t *testing.T) {
	t.Parallel()
	h := NewHandler(catalogClient())
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/user/1/cart/10", strings.NewReader("2")))
	require.Equal(t, http.StatusOK, w.Code)
}

func TestSwagger(t *testing.T) {
	t.Parallel()
	h := NewHandler(catalogClient())
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/swagger.json", nil))
	require.Equal(t, http.StatusOK, w.Code)
	require.True(t, json.Valid(w.Body.Bytes()))
	ui := httptest.NewRecorder()
	h.ServeHTTP(ui, httptest.NewRequest(http.MethodGet, "/swagger/", nil))
	require.Equal(t, http.StatusOK, ui.Code)
	require.Contains(t, ui.Body.String(), "SwaggerUIBundle")
}
