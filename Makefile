DB_URL=postgresql://root:secret@localhost:5432/task_manager?sslmode=disable

# all packages except generated mocks, comma-separated (for -coverpkg)
COVER_PKGS=$(shell go list ./... | grep -v -e /mock -e /docs | paste -sd, -)

up:
	docker compose up -d postgres

down:
	docker compose down

psql:
	docker exec -it task-postgres psql -U root -d task_manager

new_migration:
	migrate create -ext sql -dir internal/db/migration -seq $(name)

migrateup:
	migrate -path internal/db/migration -database "$(DB_URL)" -verbose up

migratedown:
	migrate -path internal/db/migration -database "$(DB_URL)" -verbose down -all

sqlc:
	sqlc generate

mock:
	mockgen -package mockdb -destination internal/db/mock/store.go $(shell go list -m)/internal/db/sqlc Store

test:
	go test -v -race -cover ./...

cover:
	go test -race -coverpkg=$(COVER_PKGS) -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out | tail -1
	go tool cover -html=coverage.out -o coverage.html

cover-check: cover
	@total=$$(go tool cover -func=coverage.out | tail -1 | awk '{print $$3}' | tr -d '%'); \
	echo "Total coverage: $$total% (minimum 70%)"; \
	awk -v t=$$total 'BEGIN { if (t < 70) { print "FAIL: coverage below 70%"; exit 1 } else { print "OK" } }'

server:
	go run ./cmd/server

compose-up:
	docker compose up -d --build

compose-down:
	docker compose down

compose-logs:
	docker compose logs -f api

swagger:
	swag fmt
	swag init -g cmd/server/main.go -o docs --parseInternal

.PHONY: up down psql new_migration migrateup migratedown sqlc mock test cover cover-check server compose-up compose-down compose-logs swagger