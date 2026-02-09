PREFIX ?= $(HOME)/.local
LFCONF ?= $(HOME)/.config/lf

all: build

build:
	go build -o aw ./cmd/aw
	go build -o alf-play ./cmd/alf-play
	go build -o alf-index ./cmd/alf-index
	go build -o alf-list ./cmd/alf-list

install: build
	install -Dm755 aw $(PREFIX)/bin/aw
	install -Dm755 alf-play $(PREFIX)/bin/alf-play
	install -Dm755 alf-index $(PREFIX)/bin/alf-index
	install -Dm755 alf-list $(PREFIX)/bin/alf-list
	install -Dm755 alf $(PREFIX)/bin/alf
	@echo "installed: aw, alf-play, alf-index, alf-list, alf"

clean:
	rm -f aw alf-play alf-index alf-list

.PHONY: all build install clean
