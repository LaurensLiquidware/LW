.PHONY: generate build test vet run

generate:
	./scripts/sync-version.sh

build: generate
	go build -o profileunity-msp-console ./cmd/server

test: generate
	go vet ./...
	go test ./...

run: generate
	go run ./cmd/server
