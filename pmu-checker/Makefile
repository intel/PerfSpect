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

pmu-checker: pmu-checker.go
	$(GO_BIN) build -o $(BINARY_NAME) -ldflags '-X main.gVersion=$(VERSION)' .

.PHONY: lint
lint:
	$(GO_LINT) .

.PHONY: test
test: pmu-checker
		sudo ./$(BINARY_NAME) | jq

.DEFAULT_GOAL := $(BINARY_NAME)
