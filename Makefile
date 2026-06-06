BINARY := audiopulse

.PHONY: build run silent run-silent test vet fmt clean doctor

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

## doctor: check the toolchain and audio stack
doctor:
	@bash scripts/doctor.sh

## clean: remove the built binary
clean:
	rm -f $(BINARY)
