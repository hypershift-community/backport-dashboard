# Variables
IMAGE_NAME := backport-dashboard
IMAGE_TAG := latest
IMAGE_FULL := $(IMAGE_NAME):$(IMAGE_TAG)

# Default target
.PHONY: all
all: build

# Build the Go application
.PHONY: build
build:
	go build -o backport-dashboard .

# Run the application
.PHONY: run
run: build
	./backport-dashboard

# Build container image using podman
.PHONY: image
image:
	podman build -t $(IMAGE_FULL) .

# Sync data from Jira
.PHONY: sync
sync: build
	./backport-dashboard --sync

# Clean build artifacts
.PHONY: clean
clean:
	rm -f backport-dashboard

# Help target
.PHONY: help
help:
	@echo "Available targets:"
	@echo "  all       - Default target, builds the application"
	@echo "  build     - Builds the Go application"
	@echo "  run       - Builds and runs the application"
	@echo "  image     - Builds a container image using podman"
	@echo "  sync      - Runs the application with the sync flag"
	@echo "  clean     - Removes build artifacts"
	@echo "  help      - Shows this help message"
