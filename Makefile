.PHONY: build test check

build:
	go build -trimpath -o garden ./cmd/eve

test:
	go test ./...

check:
	go vet ./...
	go test -race ./...
