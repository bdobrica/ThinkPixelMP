SHELL := /bin/bash

GO ?= go
GO_PACKAGES ?= ./...
GOFMT ?= gofmt
STATICCHECK_VERSION ?= v0.8.1
GOVULNCHECK_VERSION ?= v1.7.0
GO_LICENSES_VERSION ?= v2.0.1
ALLOWED_LICENSES ?= Apache-2.0,BSD-2-Clause,BSD-3-Clause,ISC,MIT
BUILD_DIR ?= /tmp/thinkpixelmp-build

.DEFAULT_GOAL := help

.PHONY: help generate fmt fmt-check vet static lint test test-unit test-race test-integration \
	test-contract test-security test-e2e openapi-generate openapi-check \
	contracts dependency-check vulnerability-check license-check build verify image

help: ## Show the stable developer and CI command surface.
	@awk 'BEGIN {FS = ":.*## "; printf "ThinkPixelMP developer targets:\n\n"} /^[a-zA-Z0-9_-]+:.*## / {printf "  %-20s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

generate: openapi-generate ## Regenerate all committed derived artifacts.

fmt: ## Format repository Go source files in place.
	$(GO) fmt ./cmd/... ./internal/... ./scripts/... ./test/...

fmt-check: ## Reject Go source files that are not gofmt-formatted.
	@test -z "$$($(GOFMT) -l cmd internal scripts test)" || { \
		printf '%s\n' 'format check failed; run make fmt'; \
		$(GOFMT) -l cmd internal scripts test; \
		exit 1; \
	}

vet: ## Run the Go vet static analysis gate.
	$(GO) vet $(GO_PACKAGES)

static: vet ## Run vet and the pinned Staticcheck analyzer.
	$(GO) run honnef.co/go/tools/cmd/staticcheck@$(STATICCHECK_VERSION) $(GO_PACKAGES)

lint: static ## Run the configured repository static analyzers.

test: ## Run all Go tests.
	$(GO) test $(GO_PACKAGES)

test-unit: test ## Run the repository unit-test gate.

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

vulnerability-check: ## Reject reachable known Go vulnerabilities.
	$(GO) run golang.org/x/vuln/cmd/govulncheck@$(GOVULNCHECK_VERSION) -test $(GO_PACKAGES)

license-check: ## Reject dependency licenses outside the policy allowlist.
	$(GO) run github.com/google/go-licenses/v2@$(GO_LICENSES_VERSION) check \
		--include_tests --allowed_licenses=$(ALLOWED_LICENSES) $(GO_PACKAGES)

build: ## Build the ThinkPixelMP service binary with reproducible paths.
	@mkdir -p $(BUILD_DIR)
	$(GO) build -trimpath -o $(BUILD_DIR)/thinkpixelmp ./cmd/thinkpixelmp

verify: fmt-check static test-unit test-race dependency-check vulnerability-check license-check openapi-check contracts build ## Run the aggregate repository gate.

image: ## Build the service image once ENG-014 supplies its definition.
	@test -f Containerfile || { printf '%s\n' 'image: unavailable until ENG-014 adds Containerfile'; exit 2; }
	docker build --file Containerfile --tag thinkpixelmp:dev .
