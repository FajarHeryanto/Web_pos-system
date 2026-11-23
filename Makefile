# POS Toko - build tools (Go + C + optional GnuCOBOL)
GO      ?= go
CC      ?= gcc
COBC    ?= cobc

ifeq ($(OS),Windows_NT)
  GO_OUT = bin\pos.exe
else
  GO_OUT = bin/pos
endif

.PHONY: all build c cobol go run clean vercel-build check

all: build

build: c cobol go

c:
	$(MAKE) -C c CC=$(CC)

c-static:
	$(MAKE) -C c CC=$(CC) static

cobol:
	$(COBC) --version >/dev/null 2>&1 || (echo "GnuCOBOL (cobc) tidak ditemukan" && exit 1)
	$(MAKE) -C cobol COBC=$(COBC)

cobol-static:
	$(MAKE) -C cobol COBC=$(COBC) static

go:
	mkdir -p bin
	CGO_ENABLED=0 $(GO) build -trimpath -ldflags="-s -w" -o $(GO_OUT) ./go

run:
	$(GO) run ./go

# Vercel dapat membangun C Engine bila gcc tersedia. COBOL opsional: tanpa
# binary COBOL terbundel, API tetap berjalan, tetapi tombol Generate COBOL
# meminta backend COBOL hybrid. Lihat README untuk penyiapan.
vercel-build:
	mkdir -p bin
	@if command -v $(CC) >/dev/null 2>&1; then $(MAKE) -C c CC=$(CC); else echo "gcc tidak tersedia; lewati C Engine. Sertakan binary di bin/ untuk mengaktifkannya."; fi
	@if command -v $(COBC) >/dev/null 2>&1; then $(MAKE) -C cobol COBC=$(COBC); else echo "cobc tidak tersedia; COBOL Reporter menggunakan alur hybrid."; fi

check:
	$(CC) -std=c99 -O2 -Wall -Wextra -pedantic -Werror -fsyntax-only c/processor.c
	$(GO) test ./...

clean:
	$(MAKE) -C c clean
	$(MAKE) -C cobol clean
	rm -f bin/pos bin/pos.exe bin/processor-linux-amd64 bin/reporter-linux-amd64
