BINDIR ?= $(HOME)/.local/bin
EVE_PARITY_FIXTURE ?= examples/eve-parity
GO_PACKAGES := $(shell go list ./... | grep -v '/node_modules/')

.PHONY: build install test test-hermetic test-race test-official test-all list-tests check

build:
	CGO_ENABLED=0 go build -trimpath -o garden ./cmd/eve

install: build
	mkdir -p "$(BINDIR)"
	install -m 0755 garden "$(BINDIR)/garden"

test: test-hermetic

test-hermetic:
	go test -count=1 $(GO_PACKAGES)

test-race:
	go test -race -count=1 $(GO_PACKAGES)

test-official:
	npm ci --prefix "$(EVE_PARITY_FIXTURE)"
	GARDEN_EVE_PARITY_FIXTURE_ROOT="$(abspath $(EVE_PARITY_FIXTURE))" \
		go test -count=1 -v ./cmd/eve \
		-run '^TestGardenBinaryHostsOfficialEveEndToEnd$$'
	GARDEN_EVE_PARITY_FIXTURE_ROOT="$(abspath $(EVE_PARITY_FIXTURE))" \
		go test -count=1 -v ./internal/evehost \
		-run '^TestOfficialEveAuthoredTypeScriptAndSandboxTerminal$$'

test-all: check test-official

list-tests:
	go test -list '^(Test|Fuzz|Benchmark)' $(GO_PACKAGES)

check:
	go vet $(GO_PACKAGES)
	$(MAKE) test-race
