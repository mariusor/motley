SHELL := bash
.ONESHELL:
.SHELLFLAGS := -eu -o pipefail -c
.DELETE_ON_ERROR:

ENV ?= dev
LDFLAGS ?= -X main.version=$(VERSION)
BUILDFLAGS ?= -trimpath -a -ldflags '$(LDFLAGS)'
TEST_FLAGS ?= -count=1
MAKEFLAGS += --warn-undefined-variables
MAKEFLAGS += --no-builtin-rules

M4 = /usr/bin/m4
M4_FLAGS =

GO := go
APPSOURCES := $(wildcard *.go internal/*/*.go cmd/*.go)
PROJECT_NAME := $(shell basename $(PWD))
TAGS := $(ENV)

export CGO_ENABLED=0

ifneq ($(ENV), dev)
	LDFLAGS += -s -w -extldflags "-static"
endif

ifeq ($(VERSION), )
	ifeq ($(shell git describe --always > /dev/null 2>&1 ; echo $$?), 0)
		BRANCH=$(shell git rev-parse --abbrev-ref HEAD | tr '/' '-')
		HASH=$(shell git rev-parse --short HEAD)
		VERSION ?= $(shell printf "%s-%s" "$(BRANCH)" "$(HASH)")
	endif
	ifeq ($(shell git describe --tags > /dev/null 2>&1 ; echo $$?), 0)
		VERSION ?= $(shell git describe --tags | tr '/' '-')
	endif
endif

BUILD := $(GO) build $(BUILDFLAGS)
TEST := $(GO) test $(BUILDFLAGS)

.PHONY: all run clean test coverage install

all: motley

motley: bin/motley
bin/motley: go.mod cmd/motley/main.go $(APPSOURCES)
	$(BUILD) -tags "$(TAGS)" -o $@ ./cmd/motley

run: ./bin/motley
	@./bin/motley

clean:
	-$(RM) bin/*

test: TEST_TARGET := ./...
test:
	$(TEST) $(TEST_FLAGS) -tags "$(TAGS)" $(TEST_TARGET)

coverage: TEST_TARGET := .
coverage: TEST_FLAGS += -covermode=count -coverprofile $(PROJECT_NAME).coverprofile
coverage: test

install: bin/motley
	install bin/motley $(DESTDIR)$(INSTALL_PREFIX)/bin
