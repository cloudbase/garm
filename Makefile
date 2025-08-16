SHELL := /bin/bash
export SHELLOPTS:=$(if $(SHELLOPTS),$(SHELLOPTS):)pipefail:errexit

.ONESHELL:

GEN_PASSWORD=$(shell (/usr/bin/apg -n1 -m32))
IMAGE_TAG = garm-build

HAS_TAILWINDCSS=$(shell (which tailwindcss || echo "no"))
IMAGE_BUILDER=$(shell (which docker || which podman))
IS_PODMAN=$(shell (($(IMAGE_BUILDER) --version | grep -q podman) && echo "yes" || echo "no"))
USER_ID=$(if $(filter yes,$(IS_PODMAN)),0,$(shell id -u))
USER_GROUP=$(if $(filter yes,$(IS_PODMAN)),0,$(shell id -g))
ROOTDIR=$(dir $(abspath $(lastword $(MAKEFILE_LIST))))
GOPATH ?= $(shell go env GOPATH)
VERSION ?= $(shell git describe --tags --match='v[0-9]*' --dirty --always)
GARM_REF ?= $(shell git rev-parse --abbrev-ref HEAD)
GO ?= go
export GARM_PASSWORD ?= ${GEN_PASSWORD}
export REPO_WEBHOOK_SECRET = ${GEN_PASSWORD}
export ORG_WEBHOOK_SECRET = ${GEN_PASSWORD}
export CREDENTIALS_NAME ?= test-garm-creds
export WORKFLOW_FILE_NAME ?= test.yml
export GARM_ADMIN_USERNAME ?= admin

ifeq ($(IS_PODMAN),yes)
    EXTRA_ARGS := -v /etc/ssl/certs/ca-certificates.crt:/etc/ssl/certs/ca-certificates.crt
endif


.PHONY: help
help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)


default: build

##@ Build

.PHONY : build-static test install-lint-deps lint go-test fmt fmtcheck verify-vendor verify create-release-files release
build-static: ## Build garm statically
	@echo Building garm
	$(IMAGE_BUILDER) build $(EXTRA_ARGS) --tag $(IMAGE_TAG) -f Dockerfile.build-static .
	mkdir -p build
	$(IMAGE_BUILDER) run --rm -e USER_ID=$(USER_ID) -e GARM_REF=$(GARM_REF) -e USER_GROUP=$(USER_GROUP) -v $(PWD)/build:/build/output:z $(IMAGE_TAG) /build-static.sh
	@echo Binaries are available in $(PWD)/build

clean: ## Clean up build artifacts
	@rm -rf ./bin ./build ./release

.PHONY: build
build: ## Build garm
	@echo Building garm ${VERSION}
	$(shell mkdir -p ./bin)
	@$(GO) build -ldflags "-s -w -X github.com/cloudbase/garm/util/appdefaults.Version=${VERSION}" -tags osusergo,netgo,sqlite_omit_load_extension -o bin/garm ./cmd/garm
	@$(GO) build -ldflags "-s -w -X github.com/cloudbase/garm/util/appdefaults.Version=${VERSION}" -tags osusergo,netgo,sqlite_omit_load_extension -o bin/garm-cli ./cmd/garm-cli
	@echo Binaries are available in $(PWD)/bin

.PHONY: build-webui
build-webui:
	@echo Building GARM web ui
	./build-webapp.sh
	rm -rf webapp/assets/_app
	cp -r webapp/build/* webapp/assets/

.PHONY: generate
generate: ## Run go generate after checking required tools are in PATH
	@echo Checking required tools...
	@which openapi-generator-cli > /dev/null || (echo "Error: openapi-generator-cli not found in PATH" && exit 1)
	@which tailwindcss > /dev/null || (echo "Error: tailwindcss not found in PATH" && exit 1)
	@echo Running go generate
	@$(GO) generate ./...

test: verify go-test ## Run tests

##@ Release
create-release-files:
	./scripts/make-release.sh

release: build-static create-release-files ## Create a release

##@ Lint / Verify
.PHONY: lint
lint: golangci-lint $(GOLANGCI_LINT) ## Run linting.
	$(GOLANGCI_LINT) run -v --build-tags=testing,integration $(GOLANGCI_LINT_EXTRA_ARGS)

.PHONY: lint-fix
lint-fix: golangci-lint $(GOLANGCI_LINT) ## Lint the codebase and run auto-fixers if supported by the linte
	GOLANGCI_LINT_EXTRA_ARGS=--fix $(MAKE) lint

verify-vendor: ## verify if all the go.mod/go.sum files are up-to-date
	$(eval TMPDIR := $(shell mktemp -d))
	@cp -R ${ROOTDIR} ${TMPDIR}/.
	@(cd ${TMPDIR}/garm && ${GO} mod tidy)
	@diff -r -u -q ${ROOTDIR} ${TMPDIR}/garm >/dev/null 2>&1; if [ "$$?" -ne 0 ];then echo "please run: go mod tidy && go mod vendor"; exit 1; fi
	@rm -rf ${TMPDIR}

verify: verify-vendor lint fmtcheck ## Run all verify-* targets

integration: build ## Run integration tests
	function cleanup {
		if [ -e "$$GITHUB_ENV" ];then
			source $$GITHUB_ENV
		fi
		./test/integration/scripts/taredown_garm.sh
		$(GO) run ./test/integration/gh_cleanup/main.go
	}
	trap cleanup EXIT
	@./test/integration/scripts/setup-garm.sh
	@$(GO) test -v ./test/integration/. -timeout=30m -tags=integration

##@ Development

go-test: ## Run tests
	@$(GO) test -race -mod=vendor -tags testing -v $(TEST_ARGS) -timeout=15m -parallel=4 -count=1 ./...

fmt: ## Run go fmt against code.
	@$(GO) fmt $$(go list ./...)


##@ Build Dependencies

## Location to install dependencies to
LOCALBIN ?= $(shell pwd)/bin
$(LOCALBIN):
	mkdir -p $(LOCALBIN)

## Tool Binaries
GOLANGCI_LINT ?= $(LOCALBIN)/golangci-lint

## Tool Versions
GOLANGCI_LINT_VERSION ?= v1.64.8

.PHONY: golangci-lint
golangci-lint: $(GOLANGCI_LINT) ## Download golangci-lint locally if necessary. If wrong version is installed, it will be overwritten.
$(GOLANGCI_LINT): $(LOCALBIN)
	test -s $(LOCALBIN)/golangci-lint && $(LOCALBIN)/golangci-lint --version | grep -q $(GOLANGCI_LINT_VERSION) || \
	GOBIN=$(LOCALBIN) go install github.com/golangci/golangci-lint/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)
