SHELL := bash

IMAGE_TAG = garm-build

USER_ID=$(shell ((docker --version | grep -q podman) && echo "0" || id -u))
USER_GROUP=$(shell ((docker --version | grep -q podman) && echo "0" || id -g))
ROOTDIR=$(dir $(abspath $(lastword $(MAKEFILE_LIST))))
GOPATH ?= $(shell go env GOPATH)
GO ?= go


default: install

.PHONY : build-static test install-lint-deps lint go-test fmt fmtcheck verify-vendor verify
build-static:
	@echo Building garm
	docker build --tag $(IMAGE_TAG) .
	docker run --rm -e USER_ID=$(USER_ID) -e USER_GROUP=$(USER_GROUP) -v $(PWD):/build/garm:z $(IMAGE_TAG) /build-static.sh
	@echo Binaries are available in $(PWD)/bin

install:
	@$(GO) install -tags osusergo,netgo,sqlite_omit_load_extension ./...
	@echo Binaries available in ${GOPATH}

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
	@gofmt -l -s $$(go list ./... | sed 's|github.com/cloudbase/garm/||g') | grep ".*\.go"; if [ "$$?" -eq 0 ]; then echo "gofmt check failed; please tun gofmt -w -s"; exit 1;fi

verify-vendor: ## verify if all the go.mod/go.sum files are up-to-date
	$(eval TMPDIR := $(shell mktemp -d))
	@cp -R ${ROOTDIR} ${TMPDIR}
	@(cd ${TMPDIR}/garm && ${GO} mod tidy)
	@diff -r -u -q ${ROOTDIR} ${TMPDIR}/garm >/dev/null 2>&1; if [ "$$?" -ne 0 ];then echo "please run: go mod tidy && go mod vendor"; exit 1; fi
	@rm -rf ${TMPDIR}

verify: verify-vendor lint fmtcheck
