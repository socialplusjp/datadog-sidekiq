PROJECT      = datadog-sidekiq
ORGANIZATION = feedforce
VERSION      = v0.0.2
SRC          ?= $(shell go list ./... | grep -v vendor)
TESTARGS     ?= -v

deps:
	docker run --rm -it \
		-v ${PWD}:/go/src/github.com/$(ORGANIZATION)/$(PROJECT) \
		-w /go/src/github.com/$(ORGANIZATION)/$(PROJECT) \
		pottava/dep ensure
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

build: deps
	go build -o build/$(PROJECT)
.PHONY: build

cross-build: deps
	rm -rf pkg
	mkdir -p pkg/dist

	docker run --rm -it \
		-v ${PWD}:/go/src/github.com/$(ORGANIZATION)/$(PROJECT) \
		-w /go/src/github.com/$(ORGANIZATION)/$(PROJECT) \
		pottava/gox:go1.10 -osarch="linux/amd64" -output="pkg/{{.OS}}_{{.Arch}}/{{.Dir}}"

	for PLATFORM in $$(find pkg -mindepth 1 -maxdepth 1 -type d); do \
		PLATFORM_NAME=$$(basename $$PLATFORM); \
		ARCHIVE_NAME=$(PROJECT)_$(VERSION)_$${PLATFORM_NAME}; \
		\
		if [ $$PLATFORM_NAME = "dist" ]; then \
			continue; \
		fi; \
		\
		pushd $$PLATFORM; \
		tar -zvcf $(CURDIR)/pkg/dist/$${ARCHIVE_NAME}.tar.gz *; \
		popd; \
	done

	pushd pkg/dist; \
	shasum -a 256 * > $(VERSION)_SHASUMS; \
	popd
.PHONY: cross-build

release: cross-build
	docker run --rm -it \
		-e GITHUB_TOKEN=${GITHUB_TOKEN} \
		-v ${PWD}:/go/src/github.com/$(ORGANIZATION)/$(PROJECT) \
		-w /go/src/github.com/$(ORGANIZATION)/$(PROJECT) \
		tsub/ghr -username $(ORGANIZATION) -repository $(PROJECT) $(VERSION) pkg/dist/
.PHONY: release
