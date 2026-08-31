.PHONY: test vet openapi-generate openapi-check contracts verify

test:
	go test ./...

vet:
	go vet ./...

openapi-generate:
	./scripts/openapi.sh generate

openapi-check:
	./scripts/openapi.sh check

contracts:
	./scripts/validate-phase0.sh

verify: test vet contracts
