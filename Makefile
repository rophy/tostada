.PHONY: help up down build test values-local
.DEFAULT_GOAL := help

CLUSTER_NAME := tostada
KUBE_CTX := kind-$(CLUSTER_NAME)
VALUES_LOCAL := charts/tostada/values-local.yaml

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*##' $(MAKEFILE_LIST) | awk -F ':.*## ' '{printf "%-15s %s\n", $$1, $$2}'

values-local: .env charts/tostada/values-local.yaml.tpl ## Generate Helm values from .env
	@set -a && . ./.env && set +a && envsubst < charts/tostada/values-local.yaml.tpl > $(VALUES_LOCAL)
	@echo "Generated $(VALUES_LOCAL)"

up: values-local ## Create cluster and deploy everything
	kind create cluster --name $(CLUSTER_NAME) --config kind-config.yaml 2>/dev/null || true
	kubectl --context $(KUBE_CTX) create namespace tostada 2>/dev/null || true
	docker compose up -d
	skaffold run --kube-context $(KUBE_CTX)

down: ## Tear down cluster and stop compose
	skaffold delete --kube-context $(KUBE_CTX)
	docker compose down
	@if kind get clusters 2>/dev/null | grep -q '^$(CLUSTER_NAME)$$'; then \
		kind delete cluster --name $(CLUSTER_NAME); \
	fi

build: ## Build Go binary and frontend
	cd web && npm run build
	go build ./cmd/tostada/

test: ## Run all tests with coverage
	go test ./... -v -coverprofile=coverage.out
	@go tool cover -func=coverage.out | tail -1
	cd web && npx vitest run --coverage
