.PHONY: build test check

build:
	CGO_ENABLED=0 go build -trimpath -o garden ./cmd/eve

test:
	go test ./...

check:
	go vet ./...
	go test -race ./...
