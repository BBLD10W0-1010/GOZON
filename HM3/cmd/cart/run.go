package main

import (
	"context"
	"errors"
	"net"
	"net/http"
	"time"

	"gozon/internal/cart"
)

func run(ctx context.Context, listener net.Listener, client cart.ProductClient) error {
	server := &http.Server{Handler: cart.NewHandler(client), ReadHeaderTimeout: 5 * time.Second}
	defer server.Close()
	shutdownDone := make(chan struct{})
	var shutdownErr error
	stop := context.AfterFunc(ctx, func() {
		defer close(shutdownDone)
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		shutdownErr = server.Shutdown(shutdownCtx)
	})
	err := server.Serve(listener)
	if !stop() {
		<-shutdownDone
	}
	if errors.Is(err, http.ErrServerClosed) {
		return shutdownErr
	}
	return err
}
