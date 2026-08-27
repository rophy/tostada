.PHONY: up dev down build test

CLUSTER_NAME := tostada

up:
	kind create cluster --name $(CLUSTER_NAME) 2>/dev/null || true
	kubectl create namespace tostada 2>/dev/null || true
	skaffold run

dev:
	kind create cluster --name $(CLUSTER_NAME) 2>/dev/null || true
	kubectl create namespace tostada 2>/dev/null || true
	skaffold dev

down:
	skaffold delete
	kind delete cluster --name $(CLUSTER_NAME)

build:
	cd web && npm run build
	go build ./cmd/tostada/

test:
	go test ./... -v
