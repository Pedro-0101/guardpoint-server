.PHONY: build run test lint clean dev db-up db-down migrate-up migrate-down

APP_NAME = server
BUILD_DIR = bin

build:
	go build -o $(BUILD_DIR)/$(APP_NAME) ./cmd/server/

run:
	go run ./cmd/server/

test:
	go test ./... -v -race -coverprofile=coverage.out

lint:
	golangci-lint run ./...

clean:
	rm -rf $(BUILD_DIR)/

dev:
	air

db-up:
	docker compose up -d

db-down:
	docker compose down

migrate-up:
	migrate -path migrations -database "$(DATABASE_URL)" up

migrate-down:
	migrate -path migrations -database "$(DATABASE_URL)" down

sqlc:
	sqlc generate
