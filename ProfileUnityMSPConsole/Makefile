.PHONY: generate frontend build test vet run

# Angular build output lands in web/dist (outputPath in angular.json),
# which internal/web embeds via go:embed. It's git-ignored, so any Go
# target that touches the httpapi/web packages needs this to have run at
# least once against the current checkout.
frontend:
	cd web/frontend && npm ci && npm run build

generate:
	./scripts/sync-version.sh
	./scripts/sync-legal.sh

build: generate frontend
	go build -o profileunity-msp-console ./cmd/server

# Explicit, not ./..., because web/frontend/node_modules ships at least
# one vendored .go file of its own (a Node package's Go implementation)
# with no go.mod to bound it out of our module — ./... would try to
# build/vet/test it too.
GO_PACKAGES := ./cmd/... ./internal/... ./web

test: generate frontend
	go vet $(GO_PACKAGES)
	go test $(GO_PACKAGES)

run: generate frontend
	go run ./cmd/server
