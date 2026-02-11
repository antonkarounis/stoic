.PHONY: run dev setup sqlc rename help

## Run the application
run: sqlc 
	go run cmd/app/main.go

## Start dev services (Postgres, Keycloak, pgAdmin)
dev: 
	cd dev && docker compose -f dev-docker-compose.yaml up -d

## Regenerate SQLC code
sqlc:
	rm -f ./internal/platform/db/gen/*.go
	cd ./internal/platform/db && sqlc generate

## Rename the Go module to your own path
rename: 
	@read -p "New module path (e.g. github.com/you/myproject): " mod; \
	grep -rl 'github.com/antonkarounis/stoic/' --include='*.go' . | xargs sed -i "s|github.com/antonkarounis/stoic/|$$mod/|g"; \
	go mod edit -module $$mod; \
	go mod tidy; \
	echo "Done. Module renamed to $$mod"

.DEFAULT_GOAL := run
