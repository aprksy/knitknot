VERSION := $(shell git describe --tags --abbrev=0 2>/dev/null || echo "v0.0.1")
COMMIT  := $(shell git rev-parse HEAD)
BUILTAT := $(shell date -u +%Y-%m-%d)

LDFLAGS := -X github.com/aprksy/knitknot/cmd.version=$(VERSION) \
           -X github.com/aprksy/knitknot/cmd.commit=$(COMMIT) \
           -X github.com/aprksy/knitknot/cmd.buildDate=$(BUILTAT)

.PHONY: build install test lint vet fmt-check fmt run clean help

## build: compile the CLI
build:
	go build -ldflags "$(LDFLAGS)" -o knitknot .

## install: install the CLI into GOPATH/bin
install:
	go install -ldflags "$(LDFLAGS)" .

## test: run all tests
test:
	go test ./... -count=1

## test-race: run all tests under the race detector
test-race:
	go test ./... -race -count=1

## lint: static analysis + formatting check (go vet + gofmt)
lint: vet fmt-check

## vet: go vet across all packages
vet:
	go vet ./...

## fmt-check: fail if any file is not gofmt-clean
fmt-check:
	@unformatted="$$(gofmt -l . | grep -v '^vendor/' | grep -v '^\.slim/' || true)"; \
	if [ -n "$$unformatted" ]; then \
		echo "These files are not gofmt-clean:"; \
		echo "$$unformatted"; \
		exit 1; \
	fi

## fmt: rewrite files with gofmt
fmt:
	gofmt -w .

## run: run the CLI
run: build
	./knitknot

## clean: remove build artefacts
clean:
	rm -f knitknot
	rm -rf coverage.html merged.coverprofile

## help: list available targets
help:
	@grep -E '^## ' $(MAKEFILE_LIST) | sed 's/^## /  make /' | sed 's/:/ —/'
