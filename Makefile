# Sample Makefile

# Variables
BINARY_NAME=bin/clio
SRC_DIR=.

# Targets
all: build

gorelease:
	goreleaser build --single-target --clean --snapshot

build:
	go build -o $(BINARY_NAME) $(SRC_DIR)

clean:
	rm -f $(BINARY_NAME)

.PHONY: all build clean gorelease
