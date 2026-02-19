.PHONY: run dev-start dev-stop sqlc rename setup air

## Install dev dependencies
setup:
	go install github.com/air-verse/air@latest


## Run the application
run: setup sqlc
	air
# go run cmd/app/main.go

## Start dev services (Postgres, Keycloak, pgAdmin)
dev-start:
	cd dev && docker compose -f dev-docker-compose.yaml up -d

dev-stop:
	cd dev && docker compose -f dev-docker-compose.yaml down 

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

clean:
	sudo rm -rf dev/data/postgres



.DEFAULT_GOAL := run
