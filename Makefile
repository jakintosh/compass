GO := go
APP := compass
BIN_DIR := ./bin
BIN := $(BIN_DIR)/$(APP)
DATA_DIR := ./data

ADDR ?= :8080
DB_PATH ?= $(DATA_DIR)/compass.db
DEV_KEY_PATH ?= $(DATA_DIR)/dev.key

.DEFAULT_GOAL := help

.PHONY: help build generate test fmt vet lint install init run clean reset

help:
	printf "Targets:\n"
	printf "  build    Build $(BIN)\n"
	printf "  generate Refresh generated source\n"
	printf "  test     Run tests\n"
	printf "  fmt      Format Go source\n"
	printf "  vet      Run Go static analysis\n"
	printf "  lint     Run formatting and static checks\n"
	printf "  install  Install the CLI binary\n"
	printf "  init     Create local runtime directories\n"
	printf "  run      Run the local service in dev auth mode\n"
	printf "  clean    Remove build and test artifacts\n"
	printf "  reset    Remove local runtime state\n"

build: generate
	mkdir -p $(BIN_DIR)
	$(GO) build -o $(BIN) ./cmd/$(APP)

generate:
	$(GO) generate ./...

test:
	$(GO) test ./...

fmt:
	$(GO) fmt ./...

vet:
	$(GO) vet ./...

lint: fmt vet

install:
	$(GO) install ./cmd/$(APP)

init:
	mkdir -p $(DATA_DIR)

run: build init
	$(BIN) serve --dev --addr $(ADDR) --db-path $(DB_PATH) --dev-key-path $(DEV_KEY_PATH)

clean:
	rm -rf $(BIN_DIR)

reset:
	rm -rf $(DATA_DIR)
