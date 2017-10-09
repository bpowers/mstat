
PREFIX   = /usr
BINDIR   = $(PREFIX)/bin


# quiet output, but allow us to look at what commands are being
# executed by passing 'V=1' to make, without requiring temporarily
# editing the Makefile.
ifneq ($V, 1)
MAKEFLAGS += -s
endif

# GNU make, you are the worst.
.SUFFIXES:
%: %,v
%: RCS/%,v
%: RCS/%
%: s.%
%: SCCS/s.%

all: mstat

mstat:
	@echo "  GO      $@"
	go build

install:
	install -D -m 4755 -o root mstat $(DESTDIR)$(BINDIR)/mstat

.PHONY: all mstat
