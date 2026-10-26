SHELL := sh

.PHONY: generate run-all test test-race test-api bench

generate:
	protoc --go_out=. --go_opt=module=gozon --go-grpc_out=. --go-grpc_opt=module=gozon api/proto/product.proto

run-all:
	@set -e; \
	mkdir -p bin; \
	product_pid=; cart_pid=; \
	trap 'kill $$product_pid $$cart_pid 2>/dev/null || true; wait $$product_pid $$cart_pid 2>/dev/null || true' EXIT; \
	trap 'exit 130' INT TERM; \
	go build -o bin/product-service.exe ./cmd/product-service; \
	go build -o bin/cart.exe ./cmd/cart; \
	./bin/product-service.exe & product_pid=$$!; \
	./bin/cart.exe & cart_pid=$$!; \
	wait

test:
	go test ./...

test-race:
	go test -race ./...

test-api:
	go test -v -count=1 -tags=api ./test/

bench:
	go test -run='^$$' -bench=. -benchmem -count=3 ./internal/product ./internal/cart
