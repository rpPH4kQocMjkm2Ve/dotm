.PHONY: build install uninstall test test-root clean

PREFIX   = /usr
DESTDIR  =
pkgname  = dotm

BINDIR     = $(PREFIX)/bin
LICENSEDIR = $(PREFIX)/share/licenses/$(pkgname)

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)

BINARY = dotm

build:
	CGO_ENABLED=0 go build -trimpath -buildmode=pie -ldflags "-X main.version=$(VERSION)" -o $(BINARY) ./cmd/dotm/

test:
	go test ./...

test-root:
	sudo go test ./internal/perms/ -v -count=1

clean:
	rm -f $(BINARY)

install: build
	install -Dm755 $(BINARY) $(DESTDIR)$(BINDIR)/$(BINARY)
	install -Dm644 LICENSE   $(DESTDIR)$(LICENSEDIR)/LICENSE

uninstall:
	rm -f  $(DESTDIR)$(BINDIR)/$(BINARY)
	rm -rf $(DESTDIR)$(LICENSEDIR)/
