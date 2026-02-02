BINARY_NAME=muna-image-google
BUILD_DIR=bin
GOPATH=$(shell go env GOPATH)

.PHONY: all build install clean

all: build install

build:
	@echo "Building..."
	@mkdir -p $(BUILD_DIR)
	go build -o $(BUILD_DIR)/$(BINARY_NAME) .
	@echo "Build complete: $(BUILD_DIR)/$(BINARY_NAME)"

install:
	@echo "Installing..."
	go install .
	@echo "Installed to $(GOPATH)/bin/$(BINARY_NAME)"

clean:
	@echo "Cleaning..."
	rm -rf $(BUILD_DIR)
	go clean
	@echo "Clean complete"
