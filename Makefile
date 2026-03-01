.PHONY: build run test generate migrate-up migrate-down docker-up docker-down clean lint

BINARY_NAME=gitlens-pro
MAIN_PATH=./cmd/server

build: generate
	go build -o bin/$(BINARY_NAME) $(MAIN_PATH)

run: generate
	go run $(MAIN_PATH)

test:
	go test ./... -v -race

test-cover:
	go test ./... -v -race -coverprofile=coverage.out
	go tool cover -html=coverage.out -o coverage.html

generate:
	templ generate

generate-watch:
	templ generate --watch

migrate-up:
	migrate -path migrations -database "mysql://$(GITLENS_DATABASE_DSN)" up

migrate-down:
	migrate -path migrations -database "mysql://$(GITLENS_DATABASE_DSN)" down 1

docker-up:
	docker compose up -d

docker-down:
	docker compose down

clean:
	rm -rf bin/ coverage.* *.coverprofile

lint:
	golangci-lint run ./...
