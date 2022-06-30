SHELL := bash

.PHONY : build-static

IMAGE_TAG = garm-build

build-static:
	@echo Building metal hub
	docker build --tag $(IMAGE_TAG) .
	docker run --rm -e USER_ID="$(shell id -u)" -e USER_GROUP="$(shell id -g)" -v $(PWD):/build/garm $(IMAGE_TAG) /build-static.sh
	@echo Binaries are available in $(PWD)/bin
