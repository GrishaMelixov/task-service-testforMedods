.PHONY: help run build test test-race cover lint tidy fmt vet docker-up docker-down docker-rebuild

BINARY := bin/taskservice
PKG    := ./...

help: ## Show this help
	@awk 'BEGIN {FS = ":.*##"; printf "Usage:\n  make <target>\n\nTargets:\n"} /^[a-zA-Z_-]+:.*?##/ { printf "  %-16s %s\n", $$1, $$2 }' $(MAKEFILE_LIST)

run: ## Run the API locally (expects DATABASE_DSN to be set)
	go run ./cmd/api

build: ## Build the API binary into ./bin
	@mkdir -p bin
	go build -o $(BINARY) ./cmd/api

test: ## Run unit tests
	go test $(PKG)

test-race: ## Run unit tests with the race detector
	go test -race $(PKG)

cover: ## Run tests with coverage report
	go test -race -coverprofile=coverage.out $(PKG)
	go tool cover -func=coverage.out

fmt: ## Format code with gofmt
	gofmt -s -w .

vet: ## Run go vet
	go vet $(PKG)

lint: ## Run golangci-lint (requires golangci-lint to be installed)
	golangci-lint run

tidy: ## Tidy go.mod / go.sum
	go mod tidy

docker-up: ## Start the full stack via docker compose
	docker compose up --build

docker-down: ## Stop the stack and remove volumes (drops the database)
	docker compose down -v

docker-rebuild: docker-down docker-up ## Rebuild the stack from scratch
