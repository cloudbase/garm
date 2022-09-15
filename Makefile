SHELL := bash

IMAGE_TAG = garm-build

USER_ID=$(shell ((docker --version | grep -q podman) && echo "0" || id -u))
USER_GROUP=$(shell ((docker --version | grep -q podman) && echo "0" || id -g))

default: build-static

.PHONY : build-static
build-static:
	@echo Building garm
	docker build --tag $(IMAGE_TAG) .
	docker run --rm -e USER_ID=$(USER_ID) -e USER_GROUP=$(USER_GROUP) -v $(PWD):/build/garm:z $(IMAGE_TAG) /build-static.sh
	@echo Binaries are available in $(PWD)/bin

.PHONY: test
test:
	go test -mod=vendor -tags testing -v $(TEST_ARGS) -timeout=15m -parallel=4 ./...
