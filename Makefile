.PHONY: test vet contracts verify

test:
	go test ./...

vet:
	go vet ./...

contracts:
	./scripts/validate-phase0.sh

verify: test vet contracts

