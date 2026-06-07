POSTGRES_USER ?= admin
POSTGRES_PASSWORD ?= password
POSTGRES_DB ?= aws
MIGRATE_DSN ?= postgres://$(POSTGRES_USER):$(POSTGRES_PASSWORD)@db:5432/$(POSTGRES_DB)?sslmode=disable
MIGRATE := docker compose run --rm migrate -path=/migrations -database "$(MIGRATE_DSN)"

dev:
	go run cmd/main.go

build:
	go build -o server cmd/main.go

fmt:
	go fmt ./...

test:
	go test ./...

compose-up:
	docker compose up -d --build

compose-down:
	docker compose down

compose-logs:
	docker compose logs -f app

migrate-up:
	$(MIGRATE) up

migrate-down:
	$(MIGRATE) down 1

migrate-status:
	$(MIGRATE) version
