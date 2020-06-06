override SHELL := /usr/bin/env bash -euxo pipefail
override .DEFAULT_GOAL := all

ifdef USE_DOCKER

%: force
	@scripts/make-with-docker.sh $@ $(MAKEFLAGS) USE_DOCKER=

Makefile:
	# a dummy target to suppress bugs of GNU Make

else # ifdef USE_DOCKER

all: force vet lint test

vet: force
	@go vet $(VETFLAGS) ./...

lint: force
	@go run golang.org/x/lint/golint -set_exit_status $(LINTFLAGS) ./...

test: force
	@go test -covermode=count -coverprofile=test/coverage.out $(TESTFLAGS) ./...
	@go tool cover -html=test/coverage.out -o test/coverage.html

endif # ifdef USE_DOCKER

.PHONY: force
force:
