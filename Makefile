
BINDIR ?= output

include build/Configfile

BEFORE_SCRIPT := $(shell build/before-make.sh)

USE_VENDORIZED_BUILD_HARNESS ?=

ifndef USE_VENDORIZED_BUILD_HARNESS
-include $(shell curl -s -H 'Authorization: token ${GITHUB_TOKEN}' -H 'Accept: application/vnd.github.v4.raw' -L https://api.github.com/repos/open-cluster-management/build-harness-extensions/contents/templates/Makefile.build-harness-bootstrap -o .build-harness-bootstrap; echo .build-harness-bootstrap)
else
-include vbh/.build-harness-vendorized
endif

default::
	@echo "Build Harness Bootstrapped"

.PHONY: deps
deps:
	go get -u github.com/golangci/golangci-lint/cmd/golangci-lint
	go get -u github.com/golang/dep/cmd/dep
	dep ensure -v

.PHONY: search-aggregator
search-aggregator:
	CGO_ENABLED=0 go build -a -v -i -installsuffix cgo -ldflags '-s -w' -o $(BINDIR)/search-aggregator ./

.PHONY: build
build: search-aggregator

.PHONY: build-linux
build-linux:
	make search-aggregator GOOS=linux

.PHONY: lint
lint:
	golangci-lint run

.PHONY: test
test:
	go test ./... -v -coverprofile cover.out

.PHONY: coverage
coverage:
	go tool cover -html=cover.out -o=cover.html

.PHONY: copyright-check
copyright-check:
	./build/copyright-check.sh

.PHONY: clean
clean::
	go clean
	rm -f cover*
	rm -rf ./$(BINDIR)
	rm -rf ./.vendor-new
	rm -rf ./vendor