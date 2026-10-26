package main

import (
	"context"
	"errors"
	"net"

	"gozon/internal/product"
	"gozon/internal/productpb"

	"google.golang.org/grpc"
)

func run(ctx context.Context, listener net.Listener, items map[int64]product.Item) error {
	server := grpc.NewServer()
	productpb.RegisterProductServiceServer(server, product.NewServer(items))
	stop := context.AfterFunc(ctx, server.GracefulStop)
	defer stop()
	defer server.Stop()
	err := server.Serve(listener)
	if errors.Is(err, grpc.ErrServerStopped) {
		return nil
	}
	return err
}
