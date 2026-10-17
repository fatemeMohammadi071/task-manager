DB_URL=postgresql://root:secret@localhost:5432/task_manager?sslmode=disable

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
	go test -v -cover ./...

server:
	go run ./cmd/server

.PHONY: up down psql new_migration migrateup migratedown sqlc mock test server