.PHONY: build run test lint clean dev db-up db-down migrate-up migrate-down docker-build docker-run docs

APP_NAME = server
BUILD_DIR = bin

build:
	go build -o $(BUILD_DIR)/$(APP_NAME) ./cmd/server/

docs:
	go run github.com/swaggo/swag/cmd/swag@v1.16.6 init -g cmd/server/main.go -o docs

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

docker-build:
	docker build -t guardpoint-server:local .

docker-run:
	docker run --rm -p 8080:8080 \
		--env-file .env \
		-e DATABASE_URL=postgres://guardpoint:guardpoint@host.docker.internal:5432/guardpoint?sslmode=disable \
		guardpoint-server:local

# --- Testes de integracao (Postgres efemero na porta 5433) ---

test-db-up:
	docker run -d --name guardpoint-test-pg \
		-e POSTGRES_PASSWORD=postgres -e POSTGRES_DB=guardpoint_test \
		-p 5433:5432 postgres:16-alpine

test-db-down:
	docker rm -f guardpoint-test-pg

test-integration:
	go test ./... -tags integration -p 1 -count=1 -race
