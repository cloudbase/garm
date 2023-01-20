SHELL := bash

IMAGE_TAG = garm-build

USER_ID=$(shell ((docker --version | grep -q podman) && echo "0" || id -u))
USER_GROUP=$(shell ((docker --version | grep -q podman) && echo "0" || id -g))
ROOTDIR=$(dir $(abspath $(lastword $(MAKEFILE_LIST))))
GO ?= go


default: build-static

.PHONY : build-static
build-static:
	@echo Building garm
	docker build --tag $(IMAGE_TAG) .
	docker run --rm -e USER_ID=$(USER_ID) -e USER_GROUP=$(USER_GROUP) -v $(PWD):/build/garm:z $(IMAGE_TAG) /build-static.sh
	@echo Binaries are available in $(PWD)/bin

.PHONY: test
test: verify go-test

go-test:
	go test -race -mod=vendor -tags testing -v $(TEST_ARGS) -timeout=15m -parallel=4 -count=1 ./...

fmtcheck:
	@gofmt -l -s $$(go list ./... | sed 's|garm/||g') | grep ".*\.go"; if [ "$$?" -eq 0 ]; then echo "gofmt check failed; please tun gofmt -w -s"; exit 1;fi

verify-vendor: ## verify if all the go.mod/go.sum files are up-to-date
	$(eval TMPDIR := $(shell mktemp -d))
	@cp -R ${ROOTDIR} ${TMPDIR}
	@(cd ${TMPDIR}/garm && ${GO} mod tidy)
	@diff -r -u -q ${ROOTDIR} ${TMPDIR}/garm >/dev/null 2>&1; if [ "$$?" -ne 0 ];then echo "please run: go mod tidy && go mod vendor"; exit 1; fi
	@rm -rf ${TMPDIR}

verify: verify-vendor fmtcheck
