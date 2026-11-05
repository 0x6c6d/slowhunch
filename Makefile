.PHONY: build test lint check

build:
	go build -o bin/slowhunch ./cmd/slowhunch

test:
	go test ./...

lint:
	@test -z "$$(gofmt -l .)" || { echo "gofmt needed for:"; gofmt -l .; exit 1; }
	go vet ./...

check: lint test build
