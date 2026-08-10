VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -X github.com/tamnd/bourbaki-solver.Version=$(VERSION)

.PHONY: all build test cover lint fmt vet install clean

# all is what CI runs, in the order CI runs it. Anything CI checks and this does
# not is a red build nobody saw coming.
all: fmt vet lint test build

build:
	go build -ldflags "$(LDFLAGS)" -o bin/bourbaki ./cmd/bourbaki

install:
	go install -ldflags "$(LDFLAGS)" ./cmd/bourbaki

test:
	go test -race ./...

cover:
	go test -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out | tail -1

fmt:
	gofmt -l -w .

vet:
	go vet ./...

lint:
	staticcheck ./...

clean:
	rm -rf bin coverage.out
