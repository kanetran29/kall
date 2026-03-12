PREFIX ?= /usr/local
VERSION ?= dev
BINARY = kall

.PHONY: build test install uninstall clean

build:
	go build -ldflags "-s -w -X main.version=$(VERSION)" -o $(BINARY) ./cmd/kall

test:
	go test -v ./...

install: build
	install -d $(PREFIX)/bin
	install -m 755 $(BINARY) $(PREFIX)/bin/$(BINARY)
	install -d $(PREFIX)/share/man/man1
	install -m 644 man/kall.1 $(PREFIX)/share/man/man1/kall.1

uninstall:
	rm -f $(PREFIX)/bin/$(BINARY)
	rm -f $(PREFIX)/share/man/man1/kall.1

clean:
	rm -f $(BINARY)
