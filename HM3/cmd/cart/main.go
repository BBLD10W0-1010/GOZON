package main

import (
	"context"
	"gozon/internal/productpb"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	productAddress := os.Getenv("PRODUCT_GRPC_TARGET")
	if productAddress == "" {
		productAddress = "localhost:50051"
	}
	conn, err := grpc.NewClient(productAddress, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()
	address := os.Getenv("CART_HTTP_ADDR")
	if address == "" {
		address = ":8080"
	}
	listener, err := net.Listen("tcp", address)
	if err != nil {
		log.Fatal(err)
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	log.Printf("cart listening on %s", listener.Addr())
	if err := run(ctx, listener, productpb.NewProductServiceClient(conn)); err != nil {
		log.Fatal(err)
	}
}
