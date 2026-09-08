.PHONY: help up down build unit-test e2e-test
.DEFAULT_GOAL := help

CLUSTER_NAME := tostada
KUBE_CTX := kind-$(CLUSTER_NAME)

help: ## Show this help
	@grep -E '^[a-zA-Z0-9_-]+:.*##' $(MAKEFILE_LIST) | awk -F ':.*## ' '{printf "%-15s %s\n", $$1, $$2}'

up: ## Create cluster and deploy everything (localhost, for local dev)
	kind create cluster --name $(CLUSTER_NAME) --config kind-config.yaml 2>/dev/null || true
	kubectl --context $(KUBE_CTX) create namespace tostada 2>/dev/null || true
	@if ! kubectl --context $(KUBE_CTX) -n tostada get secret tostada-secrets >/dev/null 2>&1; then \
		echo "Generating tostada-secrets..."; \
		kubectl --context $(KUBE_CTX) -n tostada create secret generic tostada-secrets \
			--from-literal=oidc-client-secret=tostada-secret \
			--from-literal=guacamole-json-secret-key=$$(openssl rand -hex 32) \
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
	helm unittest charts/tostada

e2e-test: ## Run e2e tests in-cluster (creates cluster if needed)
	kind create cluster --name $(CLUSTER_NAME) 2>/dev/null || true
	kubectl --context $(KUBE_CTX) create namespace tostada 2>/dev/null || true
	@if ! kubectl --context $(KUBE_CTX) -n tostada get secret tostada-secrets >/dev/null 2>&1; then \
		echo "Generating tostada-secrets..."; \
		kubectl --context $(KUBE_CTX) -n tostada create secret generic tostada-secrets \
			--from-literal=oidc-client-secret=tostada-secret \
			--from-literal=guacamole-json-secret-key=$$(openssl rand -hex 32) \
			--from-literal=hub.services.tostada.apiToken=$$(openssl rand -hex 32); \
	fi
	skaffold run --kube-context $(KUBE_CTX) -p e2e --force
	docker build -t tostada-e2e -f e2e/Dockerfile .
	kind load docker-image tostada-e2e:latest --name $(CLUSTER_NAME)
	kubectl --context $(KUBE_CTX) -n tostada delete job tostada-e2e 2>/dev/null || true
	kubectl --context $(KUBE_CTX) -n tostada apply -f e2e/job.yaml
	@echo "Waiting for e2e tests to complete..."
	@kubectl --context $(KUBE_CTX) -n tostada wait --for=condition=complete --timeout=300s job/tostada-e2e && \
		(echo "=== e2e tests PASSED ==="; kubectl --context $(KUBE_CTX) -n tostada logs job/tostada-e2e) || \
		(echo "=== e2e tests FAILED ==="; kubectl --context $(KUBE_CTX) -n tostada logs job/tostada-e2e; exit 1)
