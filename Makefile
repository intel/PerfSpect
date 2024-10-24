#!make
#
# Copyright (C) 2021-2024 Intel Corporation
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
ifneq ("$(wildcard /prebuilt/tools)","") # /prebuilt/tools is a directory in the container
	cp -r /prebuilt/tools/* internal/script/resources/x86_64
else # copy dev system tools to script resources
ifneq ("$(wildcard tools/bin)","")
		cp -r tools/bin/* internal/script/resources/x86_64
else # no prebuilt tools found
		@echo "No prebuilt tools found in /prebuilt/tools or tools/bin"
endif
endif


# Build the distribution package
.PHONY: dist
dist: resources check perfspect
	rm -rf dist/perfspect
	mkdir -p dist/perfspect/tools/x86_64
	cp LICENSE dist/perfspect/
	cp THIRD_PARTY_PROGRAMS dist/perfspect/
	cp NOTICE dist/perfspect/
	cp targets.yaml dist/perfspect/
	cp perfspect dist/perfspect/
	cd dist && tar -czf perfspect_$(VERSION_NUMBER).tgz perfspect
	cd dist && md5sum perfspect_$(VERSION_NUMBER).tgz > perfspect_$(VERSION_NUMBER).tgz.md5.txt
	rm -rf dist/perfspect
	echo '{"version": "$(VERSION_NUMBER)", "date": "$(COMMIT_DATE)", "time": "$(COMMIT_TIME)", "commit": "$(COMMIT_ID)" }' | jq '.' > dist/manifest.json
ifneq ("$(wildcard /prebuilt)","") # /prebuilt is a directory in the container
	cp -r /prebuilt/oss_source* dist/
endif

# Run package-level unit tests
.PHONY: test
test:
	go test -v ./...

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
	@echo "Checking license headers..."
	@for f in `find . -type f ! -path './perfspect_202*' ! -path './tools/bin/*' ! -path './internal/script/resources/*' ! -path './scripts/.venv/*' ! -path './test/output/*' ! -path './debug_out/*' \( -name "*.go" -o -name "*.s" -o -name "*.html" -o -name "Makefile" -o -name "*.sh" -o -name "*.Dockerfile" -o -name "*.py" \)`; do \
		if ! grep -E 'Copyright \(C\) [0-9]{4}-[0-9]{4} Intel Corporation' "$$f" >/dev/null; then echo "Error: license not found: $$f"; fail=1; fi; \
	done; if [ -n "$$fail" ]; then exit 1; fi

.PHONY: check
check: check_format check_vet check_static check_license

.PHONY: clean
clean:
	@echo "Cleaning up..."
	rm -f perfspect
	sudo rm -rf dist
	rm -rf internal/script/resources/x86_64/*
	rm -rf perfspect_2024-*
	rm -rf debug_out/*
	rm -rf test/output
