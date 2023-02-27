.PHONY: help
help:
	@echo "lint             run lint"
	@echo "release-all      compile for all platforms "
	@echo "build            build"

PROJECT=zkcli
VERSION=$(shell cat main.go |grep 'version = "[0-9]\+.[0-9]\+.[0-9]\+"' | awk -F '"' '{print $$2}')
GIT_COMMIT=$(shell git rev-parse --short HEAD)
BUILT_TIME=$(shell date -u '+%FT%T%z')

GOVERSION=$(shell go version)
GOOS=$(word 1,$(subst /, ,$(lastword $(GOVERSION))))
GOARCH=$(word 2,$(subst /, ,$(lastword $(GOVERSION))))
LDFLAGS="-X main.gitCommit=${GIT_COMMIT} -X main.built=${BUILT_TIME}"

ARCNAME=$(PROJECT)-$(VERSION)-$(GOOS)-$(GOARCH)
RELDIR=$(ARCNAME)

DISTDIR=dist
export GO111MODULE=on

.PHONY: release
release:
	rm -rf $(DISTDIR)/$(RELDIR)
	mkdir -p $(DISTDIR)/$(RELDIR)
	go clean
	GOOS=$(GOOS) GOARCH=$(GOARCH) make build
	cp $(PROJECT)$(SUFFIX_EXE) $(DISTDIR)/$(RELDIR)/
	tar czf $(DISTDIR)/$(ARCNAME).tar.gz -C $(DISTDIR) $(RELDIR)
	go clean

.PHONY: release-all
release-all:
	@$(MAKE) release GOOS=linux   GOARCH=amd64
	@$(MAKE) release GOOS=linux   GOARCH=386
	@$(MAKE) release GOOS=linux   GOARCH=arm64
	@$(MAKE) release GOOS=darwin  GOARCH=amd64
	@$(MAKE) release GOOS=darwin  GOARCH=arm64

.PHONY: build
build:
	CGO_ENABLED=0 go build -ldflags ${LDFLAGS}

.PHONY: test
test:
	go test ./...

.PHONY: lint
lint: install-tools
	gofmt -s -w .
	go vet
	golangci-lint -v run --allow-parallel-runners ./...

.PHONY: install-tools
install-tools:
	go install golang.org/x/tools/cmd/goimports@latest
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

.PHONY: upgrade-deps
upgrade-deps:
	go get -u ./...
	go mod tidy
