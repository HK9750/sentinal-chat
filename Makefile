
up:
	docker compose up -d

down:
	docker compose down

logs:
	docker compose logs -f

run:
	go run cmd/api/main.go

migrate-up:
	# Placeholder for migration command
	@echo "Running migrations..."

.PHONY: up down logs run migrate-up
