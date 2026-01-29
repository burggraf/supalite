.PHONY: build run test test-verbose clean init serve install-gotrue

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
