.PHONY: build test lint fmt vet ci

GO      ?= go
BINARY  := renobot

## build: compile the renobot binary
build:
	$(GO) build -trimpath -ldflags="-s -w" -o $(BINARY) .

## test: run all unit tests
test:
	$(GO) test ./...

## vet: run go vet static analysis
vet:
	$(GO) vet ./...

## fmt: apply gofmt formatting to all Go source files
fmt:
	gofmt -w .

## lint: check formatting and run go vet (fails if any file is not gofmt-clean)
lint:
	@unformatted=$$(gofmt -l .); \
	if [ -n "$$unformatted" ]; then \
		echo "The following files are not gofmt-clean:"; \
		echo "$$unformatted"; \
		exit 1; \
	fi
	$(GO) vet ./...

## ci: run all CI checks (lint + test + build)
ci: lint test build
