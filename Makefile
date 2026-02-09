PREFIX ?= $(HOME)/.local
LFCONF ?= $(HOME)/.config/lf

all: build

build:
	go build -o aw ./cmd/aw
	go build -o alf-play ./cmd/alf-play
	go build -o alf-index ./cmd/alf-index

install: build
	install -Dm755 aw $(PREFIX)/bin/aw
	install -Dm755 alf-play $(PREFIX)/bin/alf-play
	install -Dm755 alf-index $(PREFIX)/bin/alf-index
	install -Dm755 alf $(PREFIX)/bin/alf
	install -Dm755 alf-scope $(LFCONF)/alf-scope
	install -Dm644 alf-rc $(LFCONF)/alf-rc
	@echo "installed: aw, alf-play, alf-index, alf, alf-scope, alf-rc"

clean:
	rm -f aw alf-play alf-index

.PHONY: all build install clean
