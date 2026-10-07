BIN := overlook
PREFIX ?= $(HOME)/.local/bin
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)

.PHONY: build install run test

build:
	go build -ldflags "-X main.version=$(VERSION)" -o $(BIN) .

install: build
	mkdir -p $(PREFIX)
	cp $(BIN) $(PREFIX)/$(BIN)

run: build
	./$(BIN)

test:
	go vet ./...
	go test ./...
