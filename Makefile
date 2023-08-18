SHELL := bash

IMAGE_TAG = garm-build

USER_ID=$(shell ((docker --version | grep -q podman) && echo "0" || id -u))
USER_GROUP=$(shell ((docker --version | grep -q podman) && echo "0" || id -g))
ROOTDIR=$(dir $(abspath $(lastword $(MAKEFILE_LIST))))
GOPATH ?= $(shell go env GOPATH)
VERSION ?= $(shell git describe --tags --match='v[0-9]*' --dirty --always)
GARM_REF ?= $(shell git rev-parse --abbrev-ref HEAD)
GO ?= go


default: build

.PHONY : build-static test install-lint-deps lint go-test fmt fmtcheck verify-vendor verify create-release-files release
build-static:
	@echo Building garm
	docker build --tag $(IMAGE_TAG) -f Dockerfile.build-static .
	docker run --rm -e USER_ID=$(USER_ID) -e GARM_REF=$(GARM_REF) -e USER_GROUP=$(USER_GROUP) -v $(PWD)/build:/build/output:z $(IMAGE_TAG) /build-static.sh
	@echo Binaries are available in $(PWD)/build

create-release-files:
	./scripts/make-release.sh

release: build-static create-release-files

clean:
	@rm -rf ./bin ./build ./release

build:
	@echo Building garm ${VERSION}
	$(shell mkdir -p ./bin)
	@$(GO) build -ldflags "-s -w -X main.Version=${VERSION}" -tags osusergo,netgo,sqlite_omit_load_extension -o bin/garm ./cmd/garm
	@$(GO) build -ldflags "-s -w -X github.com/cloudbase/garm/cmd/garm-cli/cmd.Version=${VERSION}" -tags osusergo,netgo,sqlite_omit_load_extension -o bin/garm-cli ./cmd/garm-cli
	@echo Binaries are available in $(PWD)/bin

test: verify go-test

install-lint-deps:
	@$(GO) install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

lint:
	@golangci-lint run --timeout=8m --build-tags testing

go-test:
	@$(GO) test -race -mod=vendor -tags testing -v $(TEST_ARGS) -timeout=15m -parallel=4 -count=1 ./...

fmt:
	@$(GO) fmt $$(go list ./...)

fmtcheck:
	@gofmt -l -s $$(go list ./... | sed 's|github.com/cloudbase/garm/||g') | grep ".*\.go"; if [ "$$?" -eq 0 ]; then echo "gofmt check failed; please run gofmt -w -s"; exit 1;fi

verify-vendor: ## verify if all the go.mod/go.sum files are up-to-date
	$(eval TMPDIR := $(shell mktemp -d))
	@cp -R ${ROOTDIR} ${TMPDIR}
	@(cd ${TMPDIR}/garm && ${GO} mod tidy)
	@diff -r -u -q ${ROOTDIR} ${TMPDIR}/garm >/dev/null 2>&1; if [ "$$?" -ne 0 ];then echo "please run: go mod tidy && go mod vendor"; exit 1; fi
	@rm -rf ${TMPDIR}

verify: verify-vendor lint fmtcheck
