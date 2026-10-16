SHELL := /bin/sh

APP_NAME    := cart-service
MAIN_PKG    := ./cmd/cart-service
BINARY      := bin/$(APP_NAME)
MIGRATIONS  := ./migrations
DATABASE_DSN ?= postgres://cart:cart@localhost:5432/cart_db?sslmode=disable

.PHONY: help
help: ## Показать список доступных команд
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-16s\033[0m %s\n", $$1, $$2}'

.PHONY: tidy
tidy: ## Привести go.mod/go.sum в порядок
	go mod tidy

.PHONY: build
build: ## Собрать бинарник сервиса
	go build -o $(BINARY) $(MAIN_PKG)

.PHONY: run
run: ## Запустить сервис локально
	go run $(MAIN_PKG)

.PHONY: test
test: ## Запустить все тесты
	go test -race -count=1 ./...

.PHONY: test-unit
test-unit: ## Запустить только unit-тесты (без интеграционных)
	go test -race -count=1 -short ./...

.PHONY: lint
lint: ## Запустить линтер (golangci-lint)
	golangci-lint run ./...

.PHONY: fmt
fmt: ## Форматирование кода
	gofmt -s -w .

.PHONY: docker-up
docker-up: ## Поднять стек (Postgres + Valkey + сервис)
	docker compose -f deploy/docker-compose.yml up --build -d

.PHONY: docker-down
docker-down: ## Остановить стек
	docker compose -f deploy/docker-compose.yml down -v

.PHONY: migrate-up
migrate-up: ## Применить миграции к БД (goose)
	goose -dir $(MIGRATIONS) postgres "$(DATABASE_DSN)" up

.PHONY: migrate-down
migrate-down: ## Откатить последнюю миграцию (goose)
	goose -dir $(MIGRATIONS) postgres "$(DATABASE_DSN)" down
