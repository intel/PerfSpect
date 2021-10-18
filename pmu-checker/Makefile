#!make
COMMIT_ID := $(shell git rev-parse --short=8 HEAD)
COMMIT_DATE := $(shell git show -s --format=%cd --date=short HEAD)
VERSION := $(COMMIT_DATE)_$(COMMIT_ID)

GO_BIN ?= go
GO_LINT ?= golint
GO_FMT ?= gofmt
BINARY_NAME ?= pmu-checker

.PHONY: fmt
fmt:
	ifind . -name '*.go' | \
            while read -r file; \
                do $(GO_FMT) -w -s "$$file"; \
            done

pmu-checker:
	$(GO_BIN) build -o $(BINARY_NAME) -ldflags '-X main.gVersion=$(VERSION)' ./cmd/

pmu-checker_arm64:
	GOOS=linux GOARCH=arm64 $(GO_BIN) build -o $(BINARY_NAME)_arm64 -ldflags '-X main.gVersion=$(VERSION)' ./cmd/

.PHONY: lint
lint:
	$(GO_LINT) .

.PHONY: test
test: pmu-checker
		sudo ./$(BINARY_NAME) --no-stdout | jq

.DEFAULT_GOAL := $(BINARY_NAME)
