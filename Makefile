BINARY := gh-pr-dash

.PHONY: build test lint fmt install uninstall clean

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
	gh extension remove pr-dash 2>/dev/null || true
	gh extension install .

uninstall:
	gh extension remove pr-dash

clean:
	rm -f $(BINARY)
