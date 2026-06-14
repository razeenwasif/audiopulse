BINARY := audiopulse

# Installation prefix. Override with `make install PREFIX=/usr/local` (may need sudo).
PREFIX ?= $(HOME)/.local
BINDIR ?= $(PREFIX)/bin

.PHONY: build run silent run-silent test vet fmt clean doctor install install-voice uninstall librespot spotdl ollama-model voice run-voice

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

## spotdl: install spotDL (used to export your library to local audio files)
spotdl:
	@command -v pipx >/dev/null 2>&1 && pipx install spotdl || pip install --user spotdl
	@echo "spotDL installed. FFmpeg is required for conversion (system ffmpeg is used if present)."

## voice: fetch Vosk lib+model and build with offline voice control (press 'v')
voice:
	@bash scripts/fetch-vosk.sh
	go build -tags vosk -o $(BINARY) .

## run-voice: build with voice control and launch
run-voice: voice
	./$(BINARY)

## ollama-model: pull the default model for the ':' AI assistant (needs ollama)
ollama-model:
	@command -v ollama >/dev/null 2>&1 || { echo "Install Ollama first: https://ollama.com"; exit 1; }
	ollama pull $(OLLAMA_MODEL)

OLLAMA_MODEL ?= gemma3

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

## install-voice: build with voice control (-tags vosk) and install to $(BINDIR)
install-voice: voice
	@mkdir -p "$(BINDIR)"
	@install -m 0755 $(BINARY) "$(BINDIR)/$(BINARY)"
	@echo "Installed $(BINARY) (voice+RAG) -> $(BINDIR)/$(BINARY)"
	@echo "Voice ('v') needs voice_model set to an absolute path in config.json when"
	@echo "launched outside the repo — e.g. \"$(CURDIR)/third_party/vosk/model\"."
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
