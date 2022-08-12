PROJECT      = datadog-sidekiq
ORGANIZATION = feedforce
VERSION      := $(shell grep 'const version ' main.go | sed -E 's/.*"(.+)"$$/\1/')
SRC          ?= $(shell go list ./... | grep -v vendor)
TESTARGS     ?= -v

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
