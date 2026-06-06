BINARY := audiopulse

# Installation prefix. Override with `make install PREFIX=/usr/local` (may need sudo).
PREFIX ?= $(HOME)/.local
BINDIR ?= $(PREFIX)/bin

.PHONY: build run silent run-silent test vet fmt clean doctor install uninstall librespot

## build: compile with real audio (needs libasound2-dev)
build:
	go build -o $(BINARY) .

## run: build with audio and launch
run: build
	./$(BINARY)

## silent: compile the no-audio fallback (no ALSA needed)
silent:
	go build -tags nosound -o $(BINARY) .

## run-silent: build the silent fallback and launch
run-silent: silent
	./$(BINARY)

## test: run tests using the silent backend (no device required)
test:
	go test -tags nosound ./...

## vet: static analysis
vet:
	go vet -tags nosound ./...

## fmt: format all Go sources
fmt:
	gofmt -w .

## librespot: build & install the librespot playback backend (one-time, ~10-15 min)
librespot:
	cargo install librespot --locked --no-default-features --features "alsa-backend,rustls-tls-webpki-roots"

## doctor: check the toolchain and audio stack
doctor:
	@bash scripts/doctor.sh

## install: build with audio and install to $(BINDIR) (default ~/.local/bin)
install: build
	@mkdir -p "$(BINDIR)"
	@install -m 0755 $(BINARY) "$(BINDIR)/$(BINARY)"
	@echo "Installed $(BINARY) -> $(BINDIR)/$(BINARY)"
	@case ":$$PATH:" in \
		*":$(BINDIR):"*) echo "$(BINDIR) is on your PATH — run 'audiopulse' from anywhere." ;; \
		*) echo "NOTE: $(BINDIR) is not on your PATH. Add it, e.g.:"; \
		   echo "      echo 'export PATH=\"$(BINDIR):\$$PATH\"' >> ~/.bashrc && source ~/.bashrc" ;; \
	esac

## uninstall: remove the installed binary
uninstall:
	@rm -f "$(BINDIR)/$(BINARY)"
	@echo "Removed $(BINDIR)/$(BINARY)"

## clean: remove the built binary
clean:
	rm -f $(BINARY)
