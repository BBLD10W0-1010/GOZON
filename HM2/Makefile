.PHONY: generate run-all test

generate:
	protoc --go_out=. --go_opt=module=gozon --go-grpc_out=. --go-grpc_opt=module=gozon api/proto/product.proto

run-all:
	@set -e; \
	tmp_dir=$$(mktemp -d); \
	product_pid=; cart_pid=; \
	trap 'kill $$product_pid $$cart_pid 2>/dev/null || true; rm -rf "$$tmp_dir"' EXIT; \
	trap 'exit 130' INT TERM; \
	go build -o "$$tmp_dir/product-service.exe" ./cmd/product-service; \
	go build -o "$$tmp_dir/cart.exe" ./cmd/cart; \
	"$$tmp_dir/product-service.exe" & product_pid=$$!; \
	"$$tmp_dir/cart.exe" & cart_pid=$$!; \
	wait

test:
	go test ./...
