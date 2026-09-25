#!/usr/bin/env bash
#
# Downloads the Vosk native library + a small English model into
# third_party/vosk/ for the offline voice control (built with `make voice`).
# Idempotent: skips downloads that are already present. Nothing here is committed
# to git (see third_party/vosk/.gitignore).
set -euo pipefail

VOSK_VERSION="${VOSK_VERSION:-0.3.45}"
VOSK_MODEL="${VOSK_MODEL:-vosk-model-small-en-us-0.15}"

root="$(cd "$(dirname "$0")/.." && pwd)"
dest="$root/third_party/vosk"
mkdir -p "$dest"
cd "$dest"

need() { command -v "$1" >/dev/null 2>&1 || { echo "error: '$1' is required" >&2; exit 1; }; }
need curl
need unzip

if [ -f libvosk.so ] && [ -f vosk_api.h ]; then
  echo "✓ libvosk already present"
else
  echo "↓ downloading libvosk $VOSK_VERSION…"
  curl -fsSL -o vosk-lib.zip \
    "https://github.com/alphacep/vosk-api/releases/download/v${VOSK_VERSION}/vosk-linux-x86_64-${VOSK_VERSION}.zip"
  unzip -q -o vosk-lib.zip
  cp "vosk-linux-x86_64-${VOSK_VERSION}/libvosk.so" .
  cp "vosk-linux-x86_64-${VOSK_VERSION}/vosk_api.h" .
  rm -rf vosk-lib.zip "vosk-linux-x86_64-${VOSK_VERSION}"
  echo "✓ libvosk installed"
fi

if [ -d model ]; then
  echo "✓ model already present"
else
  echo "↓ downloading model $VOSK_MODEL (~40 MB)…"
  curl -fsSL -o vosk-model.zip "https://alphacephei.com/vosk/models/${VOSK_MODEL}.zip"
  unzip -q -o vosk-model.zip
  mv "$VOSK_MODEL" model
  rm -f vosk-model.zip
  echo "✓ model installed → third_party/vosk/model"
fi

echo "Done. Build with 'make voice' and press 'v' in AudioPulse to talk."
