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

GO=CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go
GOFLAGS=-trimpath -mod=readonly -gcflags="all=-spectre=all -N -l" -asmflags="all=-spectre=all" -ldflags="-X perfspect/cmd.gVersion=$(VERSION) -s -w"

# Build the perfspect binary
.PHONY: perfspect
perfspect:
	$(GO) build $(GOFLAGS) -o $@

# Copy prebuilt tools to script resources
.PHONY: resources
resources:
	mkdir -p internal/script/resources/x86_64
	mkdir -p internal/script/resources/aarch64
ifneq ("$(wildcard /prebuilt/tools)","") # /prebuilt/tools is a directory in the container
	cp -r /prebuilt/tools/* internal/script/resources/x86_64
else # copy dev system tools to script resources
ifneq ("$(wildcard tools/bin)","")
		cp -r tools/bin/* internal/script/resources/x86_64
else # no prebuilt tools found
		@echo "No prebuilt tools found in /prebuilt/tools or tools/bin"
endif
ifneq ("$(wildcard tools/bin-aarch64)","")
		cp -r tools/bin-aarch64/* internal/script/resources/aarch64
else # no prebuilt tools found
		@echo "No prebuilt tools (aarch64) found in /prebuilt/tools or tools/bin-aarch64"
endif
endif


# Build the distribution package
.PHONY: dist
dist: resources check perfspect
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
	@test -z "$(shell gofmt -l -s ./)" || { echo "[WARN] Formatting issues detected. Resolve with 'make format'"; exit 1; }
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
	@for f in `find . -type f ! -path './perfspect_202*' ! -path './tools/bin/*' ! -path './tools/bin-aarch64/*' ! -path './internal/script/resources/*' ! -path './scripts/.venv/*' ! -path './test/output/*' ! -path './debug_out/*' ! -path './tools/perf-archive/*' ! -path './tools/avx-turbo/*' \( -name "*.go" -o -name "*.s" -o -name "*.html" -o -name "Makefile" -o -name "*.sh" -o -name "*.Dockerfile" -o -name "*.py" \)`; do \
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
check: check_format check_vet check_static check_license check_lint check_vuln check_modernize

.PHONY: sweep
sweep:
	rm -rf perfspect_2025-*
	rm -rf debug_out/*
	rm -rf test/output
	rm -f __debug_bin*.log
	rm -f perfspect.log

.PHONY: clean
clean: sweep
	@echo "Cleaning up..."
	rm -f perfspect
	sudo rm -rf dist
	rm -rf internal/script/resources/x86_64/*
