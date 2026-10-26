package main

import (
	"context"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	"gozon/internal/product"
)

func main() {
	path := os.Getenv("PRODUCTS_FILE")
	if path == "" {
		path = "products.json"
	}
	items, err := product.Load(path)
	if err != nil {
		log.Fatal(err)
	}
	address := os.Getenv("PRODUCT_GRPC_ADDR")
	if address == "" {
		address = ":50051"
	}
	listener, err := net.Listen("tcp", address)
	if err != nil {
		log.Fatal(err)
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	log.Printf("product-service listening on %s with %d products", listener.Addr(), len(items))
	if err := run(ctx, listener, items); err != nil {
		log.Fatal(err)
	}
}
