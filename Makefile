BINDIR ?= $(HOME)/.local/bin

.PHONY: build install test check

build:
	CGO_ENABLED=0 go build -trimpath -o garden ./cmd/eve

install: build
	mkdir -p "$(BINDIR)"
	install -m 0755 garden "$(BINDIR)/garden"

test:
	go test ./...

check:
	go vet ./...
	go test -race ./...
