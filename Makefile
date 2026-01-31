
up:
	docker compose up -d

down:
	docker compose down

logs:
	docker compose logs -f

run:
	go run cmd/api/main.go

migrate-up:
	go run cmd/migrate/main.go up

migrate-down:
	go run cmd/migrate/main.go down

migrate-status:
	go run cmd/migrate/main.go status

migrate-seed:
	go run cmd/migrate/main.go seed

migrate-seed-dev:
	go run cmd/migrate/main.go seed-dev

migrate-reset:
	go run cmd/migrate/main.go reset

migrate-truncate:
	go run cmd/migrate/main.go truncate

.PHONY: up down logs run migrate-up migrate-down migrate-status migrate-seed migrate-seed-dev migrate-reset migrate-truncate
