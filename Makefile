#!make
#
# Copyright (C) 2021-2025 Intel Corporation
# SPDX-License-Identifier: BSD-3-Clause
#
COMMIT_ID := $(shell git rev-parse --short=8 HEAD)
COMMIT_DATE := $(shell git show -s --format=%cd --date=short HEAD)
COMMIT_TIME := $(shell git show -s --format=%cd --date=format:'%H:%M:%S' HEAD)
VERSION_FILE := ./version.txt
VERSION_NUMBER := $(shell cat ${VERSION_FILE})
VERSION := $(VERSION_NUMBER)_$(COMMIT_DATE)_$(COMMIT_ID)

default: perfspect

GOFLAGS_COMMON=-trimpath -mod=readonly -ldflags="-X perfspect/cmd.gVersion=$(VERSION) -s -w"
GO=CGO_ENABLED=0 GOOS=linux go

# Build the perfspect binary
.PHONY: perfspect
perfspect:
	GOARCH=amd64 $(GO) build $(GOFLAGS_COMMON) -gcflags="all=-spectre=all -N -l" -asmflags="all=-spectre=all" -o $@

# Build the perfspect binary for AARCH64
.PHONY: perfspect-aarch64
perfspect-aarch64:
	GOARCH=arm64 $(GO) build $(GOFLAGS_COMMON) -o $@

# Copy prebuilt tools to script resources
.PHONY: resources
resources:
	mkdir -p internal/script/resources
ifneq ("$(wildcard /prebuilt/tools)","") # /prebuilt/tools is a directory in the container
	@echo "Copying prebuilt tools from /prebuilt/tools to script resources"
	cp -r /prebuilt/tools/* internal/script/resources/
else # copy dev system tools to script resources
ifneq ("$(wildcard tools/bin)","")
		@echo "Copying dev system tools from tools/bin to script resources"
		cp -r tools/bin/* internal/script/resources/
else # no prebuilt tools found
		@echo "No prebuilt tools found in /prebuilt/tools or tools/bin"
endif
endif

# Build the distribution package
.PHONY: dist
dist: resources check perfspect perfspect-aarch64
	rm -rf dist/perfspect
	mkdir -p dist/perfspect/tools/x86_64
	mkdir -p dist/perfspect/tools/aarch64
	cp LICENSE dist/perfspect/
	cp THIRD_PARTY_PROGRAMS dist/perfspect/
	cp NOTICE dist/perfspect/
	cp targets.yaml dist/perfspect/
	cp perfspect dist/perfspect/
	cd dist && tar -czf perfspect.tgz perfspect
	cd dist && md5sum perfspect.tgz > perfspect.tgz.md5.txt
	# for aarch64 dist, overwrite perfspect binary
	cp perfspect-aarch64 dist/perfspect/perfspect
	cd dist && tar -czf perfspect-aarch64.tgz perfspect
	cd dist && md5sum perfspect-aarch64.tgz > perfspect-aarch64.tgz.md5.txt
	rm -rf dist/perfspect
	echo '{"version": "$(VERSION_NUMBER)", "date": "$(COMMIT_DATE)", "time": "$(COMMIT_TIME)", "commit": "$(COMMIT_ID)" }' | jq '.' > dist/manifest.json
ifneq ("$(wildcard /prebuilt)","") # /prebuilt is a directory in the container
	cp -r /prebuilt/oss_source* dist/
endif

# Run package-level unit tests
.PHONY: test
test:
	@echo "Running unit tests..."
	go test -v ./...
	cd tools/stackcollapse-perf && go test -v ./...

.PHONY: update-deps
update-deps:
	@echo "Updating Go dependencies..."
	go get -u ./...
	go mod tidy

# Check code formatting
.PHONY: check_format
check_format:
	@echo "Running gofmt to check for code formatting issues..."
	@files=$$(gofmt -l -s ./); \
	if [ -n "$$files" ]; then \
		echo "[WARN] Formatting issues detected in the following files:"; \
		echo "$$files"; \
		echo "Resolve with 'make format'"; \
		exit 1; \
	fi
	@echo "gofmt detected no issues"

# Format code
.PHONY: format
format:
	@echo "Running gofmt to format code..."
	gofmt -l -w -s ./

.PHONY: check_vet
check_vet:
	@echo "Running go vet to check for suspicious constructs..."
	@test -z "$(shell go vet ./...)" || { echo "[WARN] go vet detected issues"; exit 1; }
	@echo "go vet detected no issues"

.PHONY: check_static
check_static:
	@echo "Running staticcheck to check for bugs..."
	go install honnef.co/go/tools/cmd/staticcheck@latest
	staticcheck ./...

.PHONY: check_license
check_license:
	@echo "Confirming source files have license headers..."
	@for f in `find . -type f ! -path './perfspect_202*' ! -path './tools/bin/*' ! -path './tools/bin-aarch64/*' ! -path './tools-cache/*' ! -path './internal/script/resources/*' ! -path './scripts/.venv/*' ! -path './test/output/*' ! -path './debug_out/*' ! -path './tools/perf-archive/*' ! -path './tools/avx-turbo/*' \( -name "*.go" -o -name "*.s" -o -name "*.html" -o -name "Makefile" -o -name "*.sh" -o -name "*.Dockerfile" -o -name "*.py" \)`; do \
		if ! grep -E 'SPDX-License-Identifier: BSD-3-Clause' "$$f" >/dev/null; then echo "Error: license not found: $$f"; fail=1; fi; \
	done; if [ -n "$$fail" ]; then exit 1; fi

.PHONY: check_lint
check_lint:
	@echo "Running golangci-lint to check for style issues..."
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	golangci-lint run

.PHONY: check_vuln
check_vuln:
	@echo "Running govulncheck to check for vulnerabilities..."
	go install golang.org/x/vuln/cmd/govulncheck@latest
	govulncheck ./...

.PHONY: check_sec
check_sec:
	@echo "Running gosec to check for security issues..."
	go install github.com/securego/gosec/v2/cmd/gosec@latest
	gosec ./...

.PHONY: check_semgrep
check_semgrep:
	@echo "Running semgrep to check for security issues..."
	@echo "Please install semgrep from https://semgrep.dev/docs/getting-started/installation/ if not already installed."
	@echo "Running semgrep..."
	semgrep scan

.PHONY: check_modernize
check_modernize:
	@echo "Running go-modernize to check for modernization opportunities..."
	go run golang.org/x/tools/gopls/internal/analysis/modernize/cmd/modernize@latest -test ./...

.PHONY: modernize
modernize:
	@echo "Running go-modernize to apply modernization opportunities..."
	go run golang.org/x/tools/gopls/internal/analysis/modernize/cmd/modernize@latest -fix -test ./...

.PHONY: check
check: check_format check_vet check_static check_license check_lint check_vuln test

.PHONY: sweep
sweep:
	rm -rf perfspect_202?-*
	rm -rf debug_out/*
	rm -rf test/output
	rm -f __debug_bin*.log
	rm -f perfspect.log

.PHONY: clean
clean: sweep clean-tools-cache
	@echo "Cleaning up..."
	rm -f perfspect
	rm -f perfspect-aarch64
	sudo rm -rf dist
	rm -rf internal/script/resources

.PHONY: clean-tools-cache
clean-tools-cache:
	@echo "Removing cached tool binaries..."
	rm -rf tools-cache
