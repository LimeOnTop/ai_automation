.PHONY: build run clean test

build:
	go build -o agent .

run: build
	./agent

clean:
	rm -f agent

test:
	go test ./...

deps:
	go mod download
	go mod tidy

