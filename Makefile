.PHONY: build run test lint fmt vet tidy keys

build:
	CGO_ENABLED=0 go build -o bin/server ./cmd/server

run:
	go run ./cmd/server

test:
	go test -race ./...

lint:
	golangci-lint run --timeout=5m

fmt:
	gofmt -w .

vet:
	go vet ./...

tidy:
	go mod tidy

# Generate RSA key pair for JWT (run once, then base64-encode for env)
keys:
	@mkdir -p .keys
	openssl genrsa -out .keys/jwt_private.pem 2048
	openssl rsa -in .keys/jwt_private.pem -pubout -out .keys/jwt_public.pem
	@echo ""
	@echo "Set this in your .env:"
	@echo "JWT_PRIVATE_KEY_B64=$$(base64 -w0 .keys/jwt_private.pem)"

# Quick CI check (mirrors GitHub Actions)
ci: tidy fmt vet lint test build
