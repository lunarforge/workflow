.PHONY: proto clean lint openapi redocly serve-docs all build build-gateway run run-gateway

# ─── Configuration ───
MODULE       := github.com/yourorg/flowwatch
PROTO_DIR    := proto
GEN_DIR      := gen
DOCS_DIR     := docs

# ─── Main targets ───

all: proto openapi ## Generate everything

proto: ## Generate Go + Connect-Go code from protos
	buf generate
	@echo "✓ Generated Go + Connect-Go stubs in $(GEN_DIR)/"

lint: ## Lint proto files
	buf lint
	@echo "✓ Proto lint passed"

breaking: ## Check for breaking proto changes
	buf breaking --against '.git#branch=main'
	@echo "✓ No breaking changes"

# ─── OpenAPI ───

openapi: proto ## Convert proto OpenAPI v2 → v3 for Redocly
	@# buf grpc-gateway plugin outputs Swagger v2 (openapiv2).
	@# Convert to OpenAPI v3 using a Node tool or manual step.
	@# If you have the swagger2openapi CLI installed:
	@if command -v swagger2openapi >/dev/null 2>&1; then \
		swagger2openapi $(DOCS_DIR)/flowwatch.swagger.json \
			-o $(DOCS_DIR)/openapi.yaml --yaml; \
		echo "✓ Converted to OpenAPI v3: $(DOCS_DIR)/openapi.yaml"; \
	else \
		echo "⚠  swagger2openapi not found. Install: npm i -g swagger2openapi"; \
		echo "   Using Swagger v2 spec directly: $(DOCS_DIR)/flowwatch.swagger.json"; \
	fi

redocly: openapi ## Build Redocly static docs
	@if command -v redocly >/dev/null 2>&1; then \
		redocly build-docs $(DOCS_DIR)/openapi.yaml \
			--output $(DOCS_DIR)/index.html \
			--config $(DOCS_DIR)/redocly.yaml; \
		echo "✓ Built Redocly docs: $(DOCS_DIR)/index.html"; \
	else \
		echo "⚠  redocly CLI not found. Install: npm i -g @redocly/cli"; \
	fi

serve-docs: openapi ## Serve Redocly docs in dev mode with hot-reload
	@echo "Starting Redocly dev server on http://localhost:8088 ..."
	redocly preview-docs $(DOCS_DIR)/openapi.yaml \
		--config $(DOCS_DIR)/redocly.yaml \
		--port 8088

# ─── Go ───

generate: proto ## Run go generate
	go generate ./...

build: ## Build the flowwatch server binary
	go build -o bin/flowwatch ./cmd/flowwatch

build-gateway: ## Build the gateway server binary
	go build -o bin/gateway ./cmd/gateway

run: build ## Build and run flowwatch server
	./bin/flowwatch

run-gateway: build-gateway ## Build and run gateway server
	./bin/gateway

test: ## Run tests
	go test ./... -v -race

# ─── Housekeeping ───

clean: ## Remove generated files
	rm -rf $(GEN_DIR) $(DOCS_DIR)/*.swagger.json $(DOCS_DIR)/openapi.yaml $(DOCS_DIR)/index.html bin/
	@echo "✓ Cleaned"

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2}'
