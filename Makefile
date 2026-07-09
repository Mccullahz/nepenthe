# nepenthe — build, install, and cross-compile.
#
#   make            build ./nepenthe
#   make install    build and copy to $(BINDIR)  (default ~/.local/bin)
#   make uninstall  remove the installed binary
#   make run        build and open the demo vault
#   make test       run the test suite
#   make dist       cross-compile macOS + Linux binaries into ./dist
#   make clean      remove build artifacts
#
# Override the install location with PREFIX, e.g.:
#   make install PREFIX=/usr/local        (may need sudo)

BINARY := nepenthe
GO     ?= go

PREFIX ?= $(HOME)/.local
BINDIR := $(PREFIX)/bin

# Strip debug info for a smaller, tool-sized binary.
LDFLAGS := -s -w

.PHONY: all build install uninstall run test dist clean

all: build

build:
	$(GO) build -ldflags '$(LDFLAGS)' -o $(BINARY) .

install: build
	mkdir -p $(BINDIR)
	install -m 0755 $(BINARY) $(BINDIR)/$(BINARY)
	@echo "installed $(BINDIR)/$(BINARY)"
	@case ":$$PATH:" in \
		*":$(BINDIR):"*) ;; \
		*) echo "note: $(BINDIR) is not on your PATH — add this to your shell rc:"; \
		   echo "      export PATH=\"$(BINDIR):\$$PATH\"" ;; \
	esac

uninstall:
	rm -f $(BINDIR)/$(BINARY)
	@echo "removed $(BINDIR)/$(BINARY)"

run: build
	./$(BINARY) examples/vault

test:
	$(GO) test ./...

# Cross-compile release binaries. Pure Go, so CGO stays off and these build
# from any host (e.g. Linux -> Apple Silicon).
dist:
	@mkdir -p dist
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 $(GO) build -ldflags '$(LDFLAGS)' -o dist/$(BINARY)-darwin-arm64 .
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 $(GO) build -ldflags '$(LDFLAGS)' -o dist/$(BINARY)-darwin-amd64 .
	CGO_ENABLED=0 GOOS=linux  GOARCH=arm64 $(GO) build -ldflags '$(LDFLAGS)' -o dist/$(BINARY)-linux-arm64 .
	CGO_ENABLED=0 GOOS=linux  GOARCH=amd64 $(GO) build -ldflags '$(LDFLAGS)' -o dist/$(BINARY)-linux-amd64 .
	@echo "built:" && ls -1 dist

clean:
	rm -f $(BINARY)
	rm -rf dist
