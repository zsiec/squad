.PHONY: build test lint vet tidy clean install help

GO ?= go

help:
	@echo "make build   - build squad binary to ./squad"
	@echo "make test    - run tests with race detector"
	@echo "make lint    - run golangci-lint"
	@echo "make vet     - run go vet"
	@echo "make tidy    - run go mod tidy"
	@echo "make install - go install ./cmd/squad"
	@echo "make clean   - remove build artifacts"

build:
	$(GO) build -o squad ./cmd/squad

test:
	$(GO) test -race ./...

lint:
	golangci-lint run

vet:
	$(GO) vet ./...

tidy:
	$(GO) mod tidy

install:
	$(GO) install ./cmd/squad

clean:
	rm -f squad coverage.txt
	rm -rf dist/
