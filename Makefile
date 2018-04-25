PROJECT  = datadog-sidekiq
SRC      ?= $(shell go list ./... | grep -v vendor)
TESTARGS ?= -v

deps:
	dep ensure
.PHONY: deps

test:
	go test $(SRC) $(TESTARGS)
.PHONY: test

fmt:
	go fmt $(SRC)
.PHONY: fmt

vet:
	go vet $(SRC)
.PHONY: vet

build:
	go build -o build/$(PROJECT)
.PHONY: build
