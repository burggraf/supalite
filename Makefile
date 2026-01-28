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

install-gotrue:
	go install github.com/supabase/auth/cmd/gotrue@latest
