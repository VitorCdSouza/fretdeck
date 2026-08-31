BINARY := fretdeck
PYTHON ?= python3

.PHONY: help build run test lint clean install-python

help: ## show this help
	@grep -hE '^[a-z-]+:.*?## ' $(MAKEFILE_LIST) | awk 'BEGIN{FS=":.*?## "}{printf "  %-16s %s\n", $$1, $$2}'

build: ## compile the binary into the working directory
	go build -o $(BINARY) ./cmd/fretdeck

run: ## build and open the interface
	go run ./cmd/fretdeck

test: ## go tests plus the python ones
	go test ./...
	$(PYTHON) -m pytest internal/scripts -q

lint: ## vet and gofmt
	go vet ./...
	@test -z "$$(gofmt -l cmd internal)" || { gofmt -l cmd internal; exit 1; }

install-python: ## install what the audio side needs, tests included
	$(PYTHON) -m pip install -r requirements.txt pytest

clean:
	rm -f $(BINARY)
	find internal/scripts -name '__pycache__' -type d -exec rm -rf {} +
