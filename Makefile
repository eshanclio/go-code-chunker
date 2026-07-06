.PHONY: build test fmt vet fuzz run-test

# CGo is required (tree-sitter grammars include C sources).
# Ensure cc is on PATH, or set CC.

build:
	CGO_ENABLED=1 go build ./...

test:
	CGO_ENABLED=1 go test ./...

fmt:
	go fmt ./...

vet:
	go vet ./...

fuzz:
	CGO_ENABLED=1 go test -fuzz=FuzzChunk -fuzztime=30s ./chunker/...

run-test:
	CGO_ENABLED=1 go test -run $(TEST) ./$(PKG)/...
