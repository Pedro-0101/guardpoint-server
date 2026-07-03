.PHONY: build run test lint clean dev db-up db-down migrate-up migrate-down docker-build docker-run

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

docker-build:
	docker build -t guardpoint-server:local .

docker-run:
	docker run --rm -p 8080:8080 \
		--env-file .env \
		-e DATABASE_URL=postgres://guardpoint:guardpoint@host.docker.internal:5432/guardpoint?sslmode=disable \
		guardpoint-server:local
