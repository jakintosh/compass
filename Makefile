GO := go
APP := compass
DOCKER := docker
IMAGE := $(APP):latest
BIN_DIR := ./bin
BIN := $(BIN_DIR)/$(APP)
HOST_DATA_DIR := ./data
HOST_DATA_MOUNT := $(abspath $(HOST_DATA_DIR))
CONTAINER_DATA_DIR := /app/data

HOST_PORT ?= 8080
CONTAINER_PORT ?= 80

.DEFAULT_GOAL := help

.PHONY: help build generate test fmt vet lint install init run clean reset

help:
	printf "Targets:\n"
	printf "  build    Build the Docker image $(IMAGE)\n"
	printf "  generate Refresh generated source\n"
	printf "  test     Run tests\n"
	printf "  fmt      Format Go source\n"
	printf "  vet      Run Go static analysis\n"
	printf "  lint     Run formatting and static checks\n"
	printf "  install  Install the CLI binary\n"
	printf "  init     Create local runtime directories\n"
	printf "  run      Run the Docker image in dev auth mode\n"
	printf "  clean    Remove build and test artifacts\n"
	printf "  reset    Remove local runtime state\n"

build: generate
	$(DOCKER) build -t $(IMAGE) .

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
	mkdir -p $(HOST_DATA_DIR)

run: build init
	$(DOCKER) run --rm -it \
		-p $(HOST_PORT):$(CONTAINER_PORT) \
		-v "$(HOST_DATA_MOUNT):$(CONTAINER_DATA_DIR)" \
		$(IMAGE) serve --dev --addr :$(CONTAINER_PORT) --data-dir $(CONTAINER_DATA_DIR)

clean:
	rm -rf $(BIN_DIR)

reset:
	rm -rf $(HOST_DATA_DIR)
