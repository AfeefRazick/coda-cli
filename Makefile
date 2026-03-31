BINARY     := coda
MODULE     := github.com/afeef-razick/coda-cli
CMD        := ./cmd/coda
LDFLAGS    := -s -w

.PHONY: build install clean test lint tidy

build:
	go build -ldflags "$(LDFLAGS)" -o bin/$(BINARY) $(CMD)

install:
	go install -ldflags "$(LDFLAGS)" $(CMD)

clean:
	rm -rf bin/

test:
	go test ./...

lint:
	golangci-lint run ./...

tidy:
	go mod tidy
