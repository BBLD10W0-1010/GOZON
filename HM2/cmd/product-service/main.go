package main

import (
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	"gozon/internal/product"
	"gozon/internal/productpb"

	"google.golang.org/grpc"
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
	server := grpc.NewServer()
	productpb.RegisterProductServiceServer(server, product.NewServer(items))
	go func() {
		if err := server.Serve(listener); err != nil {
			log.Printf("gRPC server stopped: %v", err)
		}
	}()
	log.Printf("product-service listening on %s with %d products", address, len(items))
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	<-signals
	server.GracefulStop()
}
