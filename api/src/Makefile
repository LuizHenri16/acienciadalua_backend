.PHONY: run build test test-integration fmt vet sqlc docker-build

# Local development (reads .env when present).
run:
	go run ./cmd/api

build:
	CGO_ENABLED=0 go build -trimpath -o bin/api ./cmd/api

test:
	go test ./...

# Requires a disposable PostgreSQL database; the tests truncate app tables.
test-integration:
	TEST_DATABASE_URL="$$TEST_DATABASE_URL" go test ./internal/services -v

fmt:
	gofmt -w .

vet:
	go vet ./...

# Regenerate internal/db from sql/queries (requires sqlc v1.27+).
sqlc:
	sqlc generate

docker-build:
	docker build -t acienciadalua-api-go .
