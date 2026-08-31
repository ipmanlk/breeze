.PHONY: dev dev-ui dev-api build build-ui build-api clean setup sqlc-gen swagger-gen api-types \
        test test-go test-race test-ui seed fmt-go fmt-ui full-build regen-all i18n-extract i18n-build lint-ui

VITE_PORT ?= 5173

# Development commands (requires global tools)
dev:
	@trap 'kill 0 2>/dev/null; exit' EXIT; \
	echo "Starting Vite dev server on port $(VITE_PORT)..."; \
	cd ui && deno task dev --port $(VITE_PORT) & \
	sleep 2; \
	echo "Starting backend with Air..."; \
	air -c .air.toml

dev-ui:
	@echo "Starting Vite dev server on port $(VITE_PORT)..."
	@cd ui && deno task dev --port $(VITE_PORT)

dev-api:
	@echo "Starting backend with Air..."
	@air -c .air.toml

# Build commands
build:
	@echo "Building UI..."
	cd ui && deno task build
	@echo "Building Go binary..."
	go build -ldflags "-X ipmanlk/plume/internal/version.Version=$$(git describe --tags --always --dirty 2>/dev/null || echo dev)" -o bin/plume ./cmd/server

# build-with-codegen regenerates sqlc + swagger + api-types before building.
# Use this after schema/handler changes so generated code is never stale.
build-with-codegen: regen-all build

build-ui:
	@echo "Building UI..."
	cd ui && deno task build
	@# vite empties dist/ on build; restore the committed .gitkeep placeholder
	@# so the embedded-dir contract (see ui/embed.go) and the tree stay intact.
	touch ui/dist/.gitkeep

build-api:
	@echo "Building Go binary..."
	go build -ldflags "-X ipmanlk/plume/internal/version.Version=$$(git describe --tags --always --dirty 2>/dev/null || echo dev)" -o bin/plume ./cmd/server

# Database and code generation
sqlc-gen:
	@echo "Generating sqlc code..."
	@command -v sqlc >/dev/null 2>&1 || { echo "sqlc is not installed. Please install it (https://docs.sqlc.dev/en/latest/overview/install.html) and try again."; exit 1; }
	@sqlc generate

swagger-gen:
	@echo "Generating Swagger docs..."
	@command -v swag >/dev/null 2>&1 || { echo "swag is not installed. Please install it (go install github.com/swaggo/swag/cmd/swag@latest) and try again."; exit 1; }
	@swag init -g cmd/server/main.go -o api/swagger --parseInternal

api-types:
	@echo "Generating TypeScript types from OpenAPI spec..."
	@cd ui && deno task api-types

.PHONY: lint-ui
lint-ui:
	@echo "Checking for .innerHTML usage in ui/src (excluding generated api/)..."
	@if grep -rn '\.innerHTML[[:space:]]*=' ui/src/ --include='*.ts' --exclude-dir=api; then \
		echo "❌ Found .innerHTML usage. Use createElement or replaceChildren instead."; exit 1; \
	else \
		echo "✅ No .innerHTML usage found."; \
	fi

# Testing
test:
	@echo "Running Go tests..."
	@go test ./internal/... -count=1

test-go:
	@echo "Running Go tests..."
	@go test ./internal/... -count=1

test-race:
	@echo "Running Go tests with race detector..."
	@go test ./internal/... -race -count=1

test-ui:
	@echo "Running UI tests..."
	@cd ui && deno task test

# Database seeding
seed:
	@echo "Seeding database with sample data (destroys existing data)..."
	@go run ./cmd/seed -force

# Setup and utilities
setup:
	@echo "Installing Go dependencies..."
	go mod tidy
	@echo "Installing UI dependencies..."
	cd ui && deno install
	@echo ""
	@echo "Global tools required (install manually):"
	@echo "  go install github.com/air-verse/air@latest              (hot-reload)"
	@echo "  go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest   (code gen)"

clean:
	rm -rf bin/ tmp/
	rm -rf ui/dist/*
	@# Remove stray binaries from earlier ad-hoc `go build`/`go test -c` runs
	rm -f server seed e2e.test handler.test plume

# Formatting
fmt-go:
	@echo "Formatting Go code..."
	@go fmt ./...

fmt-ui:
	@echo "Formatting TypeScript code..."
	@cd ui && deno fmt

# Full development workflow
full-build: clean build
	@echo "Full build complete. Binary at bin/plume"

regen-all: sqlc-gen swagger-gen api-types
	@echo "All code generation complete"

# Internationalization (see docs/i18n/)
# i18n-extract: scan ui/src for msg() calls and update src/i18n/messages/*.xlf
# i18n-build:    merge translated .xlf into runtime modules src/i18n/locales/*.js
# Run extract after adding/changing msg() strings; run build after editing .xlf files.
# NOTE: components must `import { msg } from "@lit/localize"` directly (not from @/i18n)
# or the extractor won't detect the msg() calls (brand-symbol type resolution).
i18n-extract:
	@echo "Extracting i18n messages..."
	@cd ui && deno run --allow-read --allow-write --allow-env --allow-sys --allow-net npm:@lit/localize-tools@0.8.2 extract --config=lit-localize.json

i18n-build:
	@echo "Building i18n locale modules..."
	@cd ui && deno run --allow-read --allow-write --allow-env --allow-sys --allow-net npm:@lit/localize-tools@0.8.2 build --config=lit-localize.json
