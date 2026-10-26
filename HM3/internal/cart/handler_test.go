package cart

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"gozon/internal/product"
	"gozon/internal/productpb"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
)

func TestCartThroughGRPC(t *testing.T) {
	listener := bufconn.Listen(1024 * 1024)
	grpcServer := grpc.NewServer()
	productpb.RegisterProductServiceServer(grpcServer, product.NewServer(map[int64]product.Item{
		20: {SKU: 20, Name: "Second", Price: 42},
		10: {SKU: 10, Name: "First", Price: 7},
	}))
	go func() { _ = grpcServer.Serve(listener) }()
	t.Cleanup(grpcServer.Stop)
	conn, err := grpc.NewClient("passthrough:///bufnet", grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
		return listener.Dial()
	}), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	server := httptest.NewServer(NewHandler(productpb.NewProductServiceClient(conn)))
	t.Cleanup(server.Close)
	client := server.Client()

	request := func(method, path, body string, want int) *http.Response {
		t.Helper()
		req, err := http.NewRequest(method, server.URL+path, strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		resp, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		if resp.StatusCode != want {
			t.Errorf("%s %s => %d, want %d", method, path, resp.StatusCode, want)
		}
		return resp
	}
	for _, tc := range []struct {
		method, path, body string
		want               int
	}{
		{"GET", "/user/1/cart", "", 404},
		{"GET", "/user/0/cart", "", 400},
		{"POST", "/user/0/cart/10", `{"count":1}`, 400},
		{"POST", "/user/1/cart/0", `{"count":1}`, 400},
		{"POST", "/user/1/cart/10", `{"count":0}`, 400},
		{"POST", "/user/1/cart/10", `{"count":-1}`, 400},
		{"POST", "/user/1/cart/10", `{"count":4294967296}`, 400},
		{"POST", "/user/1/cart/99", `{"count":1}`, 412},
		{"POST", "/user/1/cart/20", `{"count":1}`, 200},
		{"POST", "/user/1/cart/10", `{"count":2}`, 200},
		{"POST", "/user/1/cart/10", `3`, 200},
	} {
		_ = request(tc.method, tc.path, tc.body, tc.want).Body.Close()
	}
	resp := request("GET", "/user/1/cart", "", 200)
	defer resp.Body.Close()
	var got struct {
		Items      []Item `json:"items"`
		TotalPrice uint32 `json:"total_price"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatal(err)
	}
	if len(got.Items) != 2 || got.Items[0].SKU != 10 || got.Items[0].Count != 5 || got.Items[0].Price != 7 || got.Items[1].SKU != 20 || got.TotalPrice != 77 {
		t.Fatalf("unexpected cart: %+v", got)
	}
	resp = request("GET", "/swagger.json", "", 200)
	defer resp.Body.Close()
	var spec map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&spec); err != nil || spec["openapi"] == nil {
		t.Fatalf("invalid OpenAPI document: %v", err)
	}
}
