.PHONY: build install uninstall test test-root clean man

PREFIX   = /usr
DESTDIR  =
pkgname  = dotm

BINDIR       = $(PREFIX)/bin
LICENSEDIR   = $(PREFIX)/share/licenses/$(pkgname)
MANDIR       = $(PREFIX)/share/man
ZSH_COMPDIR  = $(PREFIX)/share/zsh/site-functions
BASH_COMPDIR = $(PREFIX)/share/bash-completion/completions

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)

BINARY = dotm

MANPAGES = man/dotm.8

build:
	CGO_ENABLED=0 go build -trimpath -buildmode=pie -ldflags "-X main.version=$(VERSION)" -o $(BINARY) ./cmd/dotm/

test:
	go test ./...

test-root:
	sudo go test ./internal/perms/ -v -count=1

man: $(MANPAGES)

man/%.8: man/%.8.md
	pandoc -s -t man -o $@ $<

clean:
	rm -f $(BINARY) $(MANPAGES)

install:
	install -Dm755 $(BINARY)          $(DESTDIR)$(BINDIR)/$(BINARY)
	install -Dm644 LICENSE            $(DESTDIR)$(LICENSEDIR)/LICENSE
	install -Dm644 completions/_dotm  $(DESTDIR)$(ZSH_COMPDIR)/_dotm
	install -Dm644 completions/dotm.bash $(DESTDIR)$(BASH_COMPDIR)/dotm
	install -Dm644 man/dotm.8         $(DESTDIR)$(MANDIR)/man8/dotm.8

uninstall:
	rm -f  $(DESTDIR)$(BINDIR)/$(BINARY)
	rm -rf $(DESTDIR)$(LICENSEDIR)/
	rm -f  $(DESTDIR)$(ZSH_COMPDIR)/_dotm
	rm -f  $(DESTDIR)$(BASH_COMPDIR)/dotm
	rm -f  $(DESTDIR)$(MANDIR)/man8/dotm.8
	@echo "Note: state files in ~/.local/state/dotm/ preserved. Remove manually if needed."
