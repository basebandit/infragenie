.PHONY: build install test lint clean snapshot release help

BIN     := infragenie
CMD     := ./cmd/infragenie
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS := -s -w -X main.version=$(VERSION)

## build: compile binary to ./bin/infragenie
build:
	@mkdir -p bin
	go build -ldflags "$(LDFLAGS)" -o bin/$(BIN) $(CMD)

## install: install binary to GOBIN / GOPATH/bin
install:
	go install -ldflags "$(LDFLAGS)" $(CMD)

## test: run full test suite
test:
	go test -race ./...

## test-cover: test with HTML coverage report
test-cover:
	go test -race -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out

## lint: run golangci-lint
lint:
	golangci-lint run ./...

## snapshot: local goreleaser snapshot (no publish)
snapshot:
	goreleaser release --snapshot --clean

## release: tag and push — goreleaser runs in CI on the tag
release:
	@test -n "$(TAG)" || (echo "usage: make release TAG=v0.1.0"; exit 1)
	git tag -a $(TAG) -m "release $(TAG)"
	git push origin $(TAG)

## clean: remove build artefacts
clean:
	rm -rf bin/ dist/ coverage.out

## help: list available targets
help:
	@grep -E '^## ' Makefile | sed 's/## //' | column -t -s ':'
