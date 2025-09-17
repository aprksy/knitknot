VERSION := $(shell git describe --tags --abbrev=0 2>/dev/null || echo "v0.0.1")
COMMIT  := $(shell git rev-parse HEAD)
BUILTAT := $(shell date -u +%Y-%m-%d)

LDFLAGS := -X github.com/aprksy/knitknot/cmd.version=$(VERSION) \
           -X github.com/aprksy/knitknot/cmd.commit=$(COMMIT) \
           -X github.com/aprksy/knitknot/cmd.buildDate=$(BUILTAT)

build:
	go build -ldflags "$(LDFLAGS)" -o knitknot .

install:
	go install -ldflags "$(LDFLAGS)" .

.PHONY: build install