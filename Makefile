.PHONY: up dev down build test port-forward

CLUSTER_NAME := tostada
KUBE_CTX := kind-$(CLUSTER_NAME)

up:
	kind create cluster --name $(CLUSTER_NAME) --config kind-config.yaml 2>/dev/null || true
	kubectl --context $(KUBE_CTX) create namespace tostada 2>/dev/null || true
	skaffold run --kube-context $(KUBE_CTX)

dev:
	kind create cluster --name $(CLUSTER_NAME) --config kind-config.yaml 2>/dev/null || true
	kubectl --context $(KUBE_CTX) create namespace tostada 2>/dev/null || true
	skaffold dev --kube-context $(KUBE_CTX)

down:
	skaffold delete --kube-context $(KUBE_CTX)
	kind delete cluster --name $(CLUSTER_NAME)

port-forward:
	@echo "Tostada:   http://localhost:8080"
	@echo "OIDC Mock: http://localhost:9090"
	@kubectl --context $(KUBE_CTX) -n tostada port-forward svc/oidc-mock 9090:8080 &
	@kubectl --context $(KUBE_CTX) -n tostada port-forward svc/tostada 8080:8080

build:
	cd web && npm run build
	go build ./cmd/tostada/

test:
	go test ./... -v -coverprofile=coverage.out
	@go tool cover -func=coverage.out | tail -1
	cd web && npx vitest run --coverage
