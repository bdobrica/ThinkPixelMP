SHELL := /bin/bash

GO ?= go
GO_PACKAGES ?= ./...

.DEFAULT_GOAL := help

.PHONY: help generate fmt vet lint test test-race test-integration \
	test-contract test-security test-e2e openapi-generate openapi-check \
	contracts dependency-check verify image

help: ## Show the stable developer and CI command surface.
	@awk 'BEGIN {FS = ":.*## "; printf "ThinkPixelMP developer targets:\n\n"} /^[a-zA-Z0-9_-]+:.*## / {printf "  %-20s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

generate: openapi-generate ## Regenerate all committed derived artifacts.

fmt: ## Format repository Go source files in place.
	$(GO) fmt ./cmd/... ./internal/... ./scripts/... ./test/...

vet: ## Run the Go vet static analysis gate.
	$(GO) vet $(GO_PACKAGES)

lint: vet ## Run the currently configured repository linters.

test: ## Run all Go tests.
	$(GO) test $(GO_PACKAGES)

test-race: ## Run all Go tests with the race detector.
	$(GO) test -race $(GO_PACKAGES)

test-integration: ## Run integration test packages.
	$(GO) test ./test/integration/...

test-contract: contracts ## Run machine-readable and Go contract tests.
	$(GO) test ./test/contract/...

test-security: ## Run adversarial and security test packages.
	$(GO) test ./test/security/... ./test/federation/...

test-e2e: ## Run end-to-end test packages.
	$(GO) test ./test/e2e/...

openapi-generate: ## Regenerate the committed OpenAPI bundle.
	./scripts/openapi.sh generate

openapi-check: ## Validate OpenAPI and reject generated-bundle drift.
	./scripts/openapi.sh check

contracts: ## Validate schemas, OpenAPI, and repository whitespace.
	./scripts/validate-phase0.sh

dependency-check: ## Enforce the repository Go dependency policy.
	$(GO) run ./scripts/dependencycheck

verify: test vet contracts ## Run the current aggregate repository gate.

image: ## Build the service image once ENG-014 supplies its definition.
	@test -f Containerfile || { printf '%s\n' 'image: unavailable until ENG-014 adds Containerfile'; exit 2; }
	docker build --file Containerfile --tag thinkpixelmp:dev .
