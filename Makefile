BIN := overlook
PREFIX ?= $(HOME)/.local/bin
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)

.PHONY: build install run test release-snapshot

build:
	go build -ldflags "-X main.version=$(VERSION)" -o $(BIN) .

# Remove before copying: overwriting a binary in place on Apple Silicon keeps
# the old code signature cached, and macOS kills the new one at launch.
install: build
	mkdir -p $(PREFIX)
	rm -f $(PREFIX)/$(BIN)
	cp $(BIN) $(PREFIX)/$(BIN)

run: build
	./$(BIN)

test:
	go vet ./...
	go test ./...

# Build every release platform into dist/ without publishing anything.
release-snapshot:
	go run github.com/goreleaser/goreleaser/v2@v2.18.2 release --snapshot --clean
