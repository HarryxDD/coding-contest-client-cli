SHELL := /bin/sh

.PHONY: fmt lint test build

fmt:
	gofmt -w $$(find . -name '*.go' -not -path './ccwatch')

lint:
	@echo 'running gofmt check'
	@files="$$(gofmt -l $$(find . -name '*.go' -not -path './ccwatch'))"; \
	if [ -n "$$files" ]; then \
		echo "gofmt needed for:"; \
		echo "$$files"; \
		exit 1; \
	fi
	go vet ./...

test:
	go test ./...

build:
	go build ./...