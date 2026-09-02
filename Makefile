.PHONY: help up down build unit-test e2e-test
.DEFAULT_GOAL := help

CLUSTER_NAME := tostada
KUBE_CTX := kind-$(CLUSTER_NAME)

help: ## Show this help
	@grep -E '^[a-zA-Z0-9_-]+:.*##' $(MAKEFILE_LIST) | awk -F ':.*## ' '{printf "%-15s %s\n", $$1, $$2}'

up: ## Create cluster and deploy everything (localhost, no external domain)
	kind create cluster --name $(CLUSTER_NAME) --config kind-config.yaml 2>/dev/null || true
	kubectl --context $(KUBE_CTX) create namespace tostada 2>/dev/null || true
	@if ! kubectl --context $(KUBE_CTX) -n tostada get secret tostada-secrets >/dev/null 2>&1; then \
		echo "Generating tostada-secrets..."; \
		kubectl --context $(KUBE_CTX) -n tostada create secret generic tostada-secrets \
			--from-literal=oidc-client-secret=tostada-secret \
			--from-literal=guacamole-json-secret-key=$$(openssl rand -hex 16) \
			--from-literal=hub.services.tostada.apiToken=$$(openssl rand -hex 32); \
	fi
	skaffold run --kube-context $(KUBE_CTX)

down: ## Tear down cluster
	skaffold delete --kube-context $(KUBE_CTX) 2>/dev/null || true
	@if kind get clusters 2>/dev/null | grep -q '^$(CLUSTER_NAME)$$'; then \
		kind delete cluster --name $(CLUSTER_NAME); \
	fi

build: ## Build Go binary and frontend
	cd web && npm run build
	go build ./cmd/tostada/

unit-test: ## Run unit tests with coverage
	go test ./... -v -coverprofile=coverage.out
	@go tool cover -func=coverage.out | tail -1
	cd web && npx vitest run --coverage

e2e-test: ## Run e2e tests and collect server coverage (requires make up)
	go test -tags e2e ./e2e/... -v -count=1
	cd e2e/web && rm -rf .nyc_output && npx playwright test
	@cd e2e/web && npx nyc report --cwd /app/web --temp-dir $(CURDIR)/e2e/web/.nyc_output --exclude-after-remap false
	@echo "Flushing coverage from server..."
	@mkdir -p coverage-e2e
	@curl -sf -X POST http://localhost:30080/debug/coverage/flush | tar xf - -C coverage-e2e
	@go tool covdata textfmt -i=coverage-e2e -o=coverage-e2e.out
	@go tool cover -func=coverage-e2e.out | tail -1
