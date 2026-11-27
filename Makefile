mkfile_path := $(abspath $(lastword $(MAKEFILE_LIST)))
mkfile_dir := $(dir $(mkfile_path))

.PHONY: build up down logs frontend seed run sh test vet fmt

# Start all services with hot reload via Docker Compose (air for the API
# server, `go run` for the MCP server, npm start for the frontend)
up:
	docker compose up --build -d
	$(MAKE) logs

# Stop all services
down:
	docker compose down

# Tail logs from all services
logs:
	docker compose logs -f

# Run the React frontend dev server locally (requires node/npm)
frontend:
	cd app/local-recipes && npm start

# Add a couple of example recipes via the API (server must be running,
# and X_AUTH_USER must match a header your dev proxy/gateway injects -
# set it to anything when hitting the backend directly without auth gating).
X_AUTH_USER ?= dev-user
seed:
	curl -sf -X POST http://localhost:8080/recipes -H "Content-Type: application/json" -H "X-Auth-Request-User: $(X_AUTH_USER)" \
		-d '{"title":"Weeknight Chili","description":"A quick, one-pot chili.","body_markdown":"## Instructions\n\n1. Brown the beef.\n2. Add beans, tomatoes, and spices.\n3. Simmer 30 minutes.","servings":6,"prep_minutes":10,"cook_minutes":40,"ingredients":[{"name":"ground beef","quantity":1,"unit":"lb"},{"name":"kidney beans","quantity":2,"unit":"can"},{"name":"diced tomatoes","quantity":1,"unit":"can"}]}'
	curl -sf -X POST http://localhost:8080/recipes -H "Content-Type: application/json" -H "X-Auth-Request-User: $(X_AUTH_USER)" \
		-d '{"title":"Simple Pancakes","description":"Fluffy breakfast pancakes.","body_markdown":"## Instructions\n\n1. Whisk dry ingredients.\n2. Whisk in milk and eggs.\n3. Cook on a griddle until golden.","servings":4,"prep_minutes":5,"cook_minutes":15,"ingredients":[{"name":"flour","quantity":2,"unit":"cup"},{"name":"milk","quantity":1.5,"unit":"cup"},{"name":"egg","quantity":2,"unit":"whole"}]}'

# Run the Go test suite
test:
	go test ./...

vet:
	go vet ./...

fmt:
	gofmt -l .

# Build the production Docker image
build:
	docker build -t local-recipes .

# Legacy single-container run targets
run:
	docker run -p 8080:8080 -p 8090:8090 local-recipes

sh:
	docker run -it -p 8080:8080 -p 8090:8090 local-recipes sh
