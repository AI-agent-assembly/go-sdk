.PHONY: fmt lint test

fmt:
	gofmt -w $(shell find . -name '*.go' -type f)

lint:
	golangci-lint run ./...

test:
	go test ./...
