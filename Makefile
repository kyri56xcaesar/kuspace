# Variables
APP_NAME := gshell
MAIN_DIR := cmd/api/
GO_FILES := $(shell find . -name '*.go' -type f)
VERSION := $(shell git describe --tags --always --dirty)
BUILD_DIR := build
BIN := $(BUILD_DIR)/$(APP_NAME)



# Commands
GO := go
GOTEST := $(GO) test
GOBUILD := $(GO) build
GOCLEAN := $(GO) clean
GOMOD := $(GO) mod
GOINSTALL := $(GO) install


# Default task
.PHONY: all
all: build



.PHONY: mod
mod:
	@echo "Downloading Go modules..."
	$(GOMOD) tidy


# Build the Go binary
.PHONY: build
build: mod
	@echo "Building $(APP_NAME)..."
	$(GOBUILD) -ldflags="-X main.Version=$(VERSION)" -o $(BIN) ./$(MAIN_DIR)main.go


# Run the application
.PHONY: run 
run: build
	@echo "Running $(APP_NAME)..."
	./$(BIN)

.PHONY: test
test:
	@echo "Running tests..."
	$(GOTEST) ./...


# Clean up
clean:
	@echo "Cleaning up..."
	$(GOCLEAN)
	rm -rf $(BUILD_DIR)

# Install the application (install locally)
.PHONY: lint
lint:
	@echo "Running linter..."
	golangci-lint run ./...


# Format the code using gofmt
.PHONY: fmt
fmt:
	@echo "Formatting code..."
	gofmt -w $(GO_FILES)


# Run a clean build
.PHONY: rebuild
rebuild: clean build


