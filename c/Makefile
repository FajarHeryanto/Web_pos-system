# Build C Processing Engine
CC      ?= gcc
CFLAGS  ?= -std=c99 -O2 -Wall -Wextra -pedantic
LDLIBS  = -lm

ifeq ($(OS),Windows_NT)
  EXE   = .exe
  MKDIR = if not exist ..\bin mkdir ..\bin
  RM    = del /Q
  TARGET = ..\bin\processor$(EXE)
else
  EXE   =
  MKDIR = mkdir -p ../bin
  RM    = rm -f
  TARGET = ../bin/processor$(EXE)
endif

.PHONY: all static clean

all: $(TARGET)

$(TARGET): processor.c processor.h
	$(MKDIR)
	$(CC) $(CFLAGS) -o $(TARGET) processor.c $(LDLIBS)

# Binary Linux statis untuk dibundel ke Vercel (jalankan di Linux x86_64 / WSL)
static: processor.c processor.h
	$(MKDIR)
	$(CC) $(CFLAGS) -static -o ../bin/processor-linux-amd64 processor.c $(LDLIBS)

clean:
	-$(RM) $(TARGET)
