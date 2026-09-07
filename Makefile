.PHONY: all build test lint clean install

all: lint test build

build:
	go build -o bin/toolkit

test:
	go test ./...

lint:
	go vet ./...

clean:
	rm -rf bin/

install:
	go build -o /usr/local/bin/toolkit
