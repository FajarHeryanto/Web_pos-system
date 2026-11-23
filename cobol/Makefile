# Build COBOL Batch Reporter (butuh GnuCOBOL 3.x: perintah `cobc`)
COBC      ?= cobc
COBFLAGS  ?= -x -free -O2

ifeq ($(OS),Windows_NT)
  MKDIR  = if not exist ..\bin mkdir ..\bin
  RM     = del /Q
  TARGET = ..\bin\reporter.exe
else
  MKDIR  = mkdir -p ../bin
  RM     = rm -f
  TARGET = ../bin/reporter
endif

.PHONY: all static clean

all: $(TARGET)

$(TARGET): reporter.cob
	$(MKDIR)
	$(COBC) $(COBFLAGS) -o $(TARGET) reporter.cob

# Percobaan link statis (butuh libcob.a, libgmp.a, dll. terpasang).
# Jika gagal, pakai alur hybrid di README.
static: reporter.cob
	$(MKDIR)
	$(COBC) $(COBFLAGS) -static -o ../bin/reporter-linux-amd64 reporter.cob

clean:
	-$(RM) $(TARGET)
