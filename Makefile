.PHONY: help up down build test e2e e2e-api local-up local-down
.DEFAULT_GOAL := help

CLUSTER_NAME := tostada
KUBE_CTX := kind-$(CLUSTER_NAME)

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*##' $(MAKEFILE_LIST) | awk -F ':.*## ' '{printf "%-15s %s\n", $$1, $$2}'

up: ## Create cluster and deploy everything (localhost, no external domain)
	kind create cluster --name $(CLUSTER_NAME) --config kind-config.yaml 2>/dev/null || true
	kubectl --context $(KUBE_CTX) create namespace tostada 2>/dev/null || true
	skaffold run --kube-context $(KUBE_CTX)

down: ## Tear down cluster
	skaffold delete --kube-context $(KUBE_CTX) 2>/dev/null || true
	@if kind get clusters 2>/dev/null | grep -q '^$(CLUSTER_NAME)$$'; then \
		kind delete cluster --name $(CLUSTER_NAME); \
	fi

local-up: ## Overlay external domain from .env (nginx proxy + HTTPS OIDC)
	@test -f .env || { echo "Error: .env not found. Copy .env.example and configure DOMAIN."; exit 1; }
	@set -a && . ./.env && set +a && envsubst < charts/tostada/values-local.yaml.tpl > charts/tostada/values-local.yaml
	@echo "Generated charts/tostada/values-local.yaml"
	docker compose up -d
	skaffold run --kube-context $(KUBE_CTX) -p local

local-down: ## Remove domain overlay, restore localhost defaults
	docker compose down 2>/dev/null || true
	skaffold run --kube-context $(KUBE_CTX)

build: ## Build Go binary and frontend
	cd web && npm run build
	go build ./cmd/tostada/

test: ## Run all tests with coverage
	go test ./... -v -coverprofile=coverage.out
	@go tool cover -func=coverage.out | tail -1
	cd web && npx vitest run --coverage

e2e: ## Run all e2e tests (requires make up)
	go test -tags e2e ./e2e/... -v -count=1

e2e-api: ## Run API e2e tests only
	go test -tags e2e ./e2e/api/ -v -count=1
