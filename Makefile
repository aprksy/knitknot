VERSION := $(shell git describe --tags --abbrev=0 2>/dev/null || echo "v0.0.1")
COMMIT  := $(shell git rev-parse HEAD)
BUILTAT := $(shell date -u +%Y-%m-%d)

LDFLAGS := -X github.com/aprksy/knitknot/cmd.version=$(VERSION) \
           -X github.com/aprksy/knitknot/cmd.commit=$(COMMIT) \
           -X github.com/aprksy/knitknot/cmd.buildDate=$(BUILTAT)

.PHONY: build install test lint vet fmt-check fmt run clean help test-cover

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

## test-cover: run all tests with coverage and fail if total is below 70%
test-cover:
	go test ./... -coverprofile=cover.out -count=1
	@go tool cover -func=cover.out | awk '{ if ($$1 == "total:") { found=1; pct=$$3; sub(/%/, "", pct); if (pct + 0 < 70) { printf "coverage %s%% below floor 70%%\n", pct; exit 1 } printf "coverage %s%% meets floor 70%%\n", pct } } END { if (!found) { print "no total coverage line found"; exit 1 } }'

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
	rm -f cover.out
	rm -rf coverage.html merged.coverprofile

## help: list available targets
help:
	@grep -E '^## ' $(MAKEFILE_LIST) | sed 's/^## /  make /' | sed 's/:/ —/'
