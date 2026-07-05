BINARY := gh-pr-dash

.PHONY: build test lint fmt install clean

build:
	go build -o $(BINARY) .

test:
	go test ./...

lint:
	test -z "$$(gofmt -l .)" || { gofmt -l .; exit 1; }
	go vet ./...

fmt:
	gofmt -w .

install: build
	gh extension install .

clean:
	rm -f $(BINARY)
