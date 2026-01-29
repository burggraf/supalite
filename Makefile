.PHONY: build run test test-verbose clean init serve install-gotrue build-gotrue-release

BINARY=supalite
VERSION=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME=$(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
GIT_COMMIT=$(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")

LDFLAGS=-ldflags "-X github.com/markb/supalite/cmd.Version=$(VERSION) -X github.com/markb/supalite/cmd.BuildTime=$(BUILD_TIME) -X github.com/markb/supalite/cmd.GitCommit=$(GIT_COMMIT)"

build:
	go build $(LDFLAGS) -o $(BINARY) .

run: build
	./$(BINARY) serve

test:
	go test -parallel=1 ./...

test-verbose:
	go test -v -parallel=1 ./...

clean:
	rm -f $(BINARY)
	rm -rf ./data

init:
	go run . init

serve:
	go run . serve

# GoTrue version to install - matches Supabase hosted auth version
GOTRUE_VERSION ?= v2.186.0

install-gotrue:
	@echo "Installing GoTrue (auth) binary version $(GOTRUE_VERSION) from source..."
	@if [ ! -f ./gotrue ]; then \
		echo "Cloning supabase/auth repository at tag $(GOTRUE_VERSION)..."; \
		git clone --depth 1 --branch $(GOTRUE_VERSION) https://github.com/supabase/auth.git /tmp/supalite-gotrue-build && \
		cd /tmp/supalite-gotrue-build && \
		make build && \
		cp ./auth $$OLDPWD/gotrue && \
		cd $$OLDPWD && \
		rm -rf /tmp/supalite-gotrue-build && \
		echo "GoTrue $(GOTRUE_VERSION) binary installed as ./gotrue"; \
	else \
		echo "GoTrue binary already exists at ./gotrue (run 'rm ./gotrue' to reinstall)"; \
	fi

# Build GoTrue binaries for all platforms (for GitHub releases)
build-gotrue-release:
	@echo "Building GoTrue binaries for all platforms..."
	@rm -rf /tmp/supalite-gotrue-release
	@mkdir -p /tmp/supalite-gotrue-release
	@echo "Cloning supabase/auth repository at tag $(GOTRUE_VERSION)..."
	@git clone --depth 1 --branch $(GOTRUE_VERSION) https://github.com/supabase/auth.git /tmp/supalite-gotrue-release/source
	@echo "Building for darwin-arm64 (Apple Silicon)..."
	@cd /tmp/supalite-gotrue-release/source && \
		GOOS=darwin GOARCH=arm64 make build && \
		mv auth ../gotrue-darwin-arm64 && \
		chmod +x ../gotrue-darwin-arm64
	@echo "Building for darwin-amd64 (Intel Mac)..."
	@cd /tmp/supalite-gotrue-release/source && \
		GOOS=darwin GOARCH=amd64 make build && \
		mv auth ../gotrue-darwin-amd64 && \
		chmod +x ../gotrue-darwin-amd64
	@echo "Building for linux-amd64..."
	@cd /tmp/supalite-gotrue-release/source && \
		GOOS=linux GOARCH=amd64 make build && \
		mv auth ../gotrue-linux-amd64 && \
		chmod +x ../gotrue-linux-amd64
	@echo "Building for linux-arm64..."
	@cd /tmp/supalite-gotrue-release/source && \
		GOOS=linux GOARCH=arm64 make build && \
		mv auth ../gotrue-linux-arm64 && \
		chmod +x ../gotrue-linux-arm64
	@echo "Binaries built in /tmp/supalite-gotrue-release:"
	@ls -lh /tmp/supalite-gotrue-release/gotrue-*
	@echo ""
	@echo "To create a GitHub release, upload these files:"
