# reference: http://www.codershaven.com/multi-platform-makefile-for-go/

HOSTOS=$(shell go env GOHOSTOS)
HOSTARCH=$(shell go env GOHOSTARCH)

EXECUTABLE=aichatconf
WINDOWS=$(EXECUTABLE)_windows_amd64.exe
LINUX=$(EXECUTABLE)_linux_amd64
DARWIN=$(EXECUTABLE)_darwin_amd64
VERSION=$(shell git describe --tags --always --long --dirty)
BUILDFLAGS=-v -trimpath -ldflags="-s -w -X main.version=$(VERSION)"

default:
	GOOS=$(HOSTOS) GOARCH=$(HOSTARCH) go build -ldflags="-X main.version=$(VERSION)" -o bin/$(EXECUTABLE)

windows: $(WINDOWS) ## Build for Windows

linux: $(LINUX) ## Build for Linux

darwin: $(DARWIN) ## Build for Darwin (macOS)

$(WINDOWS):
	env GOOS=windows GOARCH=amd64 go build $(BUILDFLAGS) -o build/$(WINDOWS)

$(LINUX):
	env GOOS=linux GOARCH=amd64 go build $(BUILDFLAGS) -o build/$(LINUX)

$(DARWIN):
	env GOOS=darwin GOARCH=amd64 go build $(BUILDFLAGS) -o build/$(DARWIN)

build: windows linux darwin ## Build binaries
	@echo version: $(VERSION)

# all: test build ## Build and run tests

# test: ## Run unit tests
# 	./scripts/test_unit.sh

clean: ## Remove previous build
	rm -f build/$(WINDOWS) build/$(LINUX) build/$(DARWIN)

help: ## Display available commands
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'

.PHONY: default windows linux darwin clean help


