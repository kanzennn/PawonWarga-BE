.PHONY: run build swagger tidy test

# Run the development server
run:
	go run main.go

# Build binary
build:
	go build -o bin/pawonwarga main.go

# Generate Swagger docs (requires: go install github.com/swaggo/swag/cmd/swag@latest)
swagger:
	swag init --parseDependency --parseInternal

# Tidy dependencies
tidy:
	go mod tidy

# Run tests
test:
	go test ./...

# Full setup: install swag, generate docs, tidy, then run
setup: tidy swagger run
