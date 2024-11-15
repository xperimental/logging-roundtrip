SHELL := /bin/bash
GO ?= go
GO_CMD := CGO_ENABLED=0 $(GO)
GIT_COMMIT := $(shell git rev-parse HEAD)
DOCKER_REPO ?= ghcr.io/xperimental/logging-roundtrip
DOCKER_TAG ?= dev

.PHONY: all
all: test build-binary

include .bingo/Variables.mk

.PHONY: test
test:
	$(GO_CMD) test -cover ./...

.PHONY: lint
lint: $(GOLANGCI_LINT)
	$(GOLANGCI_LINT) run --fix

.PHONY: build-binary
build-binary:
	$(GO_CMD) build -tags netgo -ldflags "-w -X main.GitCommit=$(GIT_COMMIT)" -o logging-roundtrip .

.PHONY: image
image:
	docker buildx build -t "$(DOCKER_REPO):$(DOCKER_TAG)" --load .

.PHONY: all-images
all-images:
	docker buildx build -t "$(DOCKER_REPO):$(DOCKER_TAG)" --platform linux/amd64,linux/arm64 --push .

.PHONY: tools
tools: $(BINGO) $(GOLANGCI_LINT)
	@echo Tools built.

.PHONY: clean
clean:
	rm -f logging-roundtrip
