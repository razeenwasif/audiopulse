#!/usr/bin/env bash
#
# AudioPulse doctor — checks the toolchain and audio stack and reports whether
# the app can be built with audio and whether sound can actually be produced.
#
# Run via: make doctor
set -u

pass=0
warn=0
fail=0

ok()   { printf '  \033[32m✓\033[0m %s\n' "$1"; pass=$((pass + 1)); }
note() { printf '  \033[33m!\033[0m %s\n' "$1"; warn=$((warn + 1)); }
bad()  { printf '  \033[31m✗\033[0m %s\n' "$1"; fail=$((fail + 1)); }

printf '\n\033[1m♫ AudioPulse doctor\033[0m\n'

# --- Toolchain ---------------------------------------------------------------
printf '\nToolchain\n'
if command -v go >/dev/null 2>&1; then
  ok "Go $(go version | awk '{print $3}' | sed 's/^go//')"
else
  bad "Go not found — install Go 1.22+ (https://go.dev/dl/)"
fi
if command -v gcc >/dev/null 2>&1 || command -v cc >/dev/null 2>&1; then
  ok "C compiler present (required by the audio backend / cgo)"
else
  note "No C compiler — the real audio backend won't build; the silent build still works"
fi

# --- Build dependency (Linux ALSA headers) -----------------------------------
printf '\nAudio build dependency\n'
have_alsa_dev=0
if [ "$(uname)" != "Linux" ]; then
  ok "Non-Linux platform — no ALSA headers required"
  have_alsa_dev=1
elif pkg-config --exists alsa 2>/dev/null; then
  ok "ALSA dev headers present (alsa $(pkg-config --modversion alsa))"
  have_alsa_dev=1
else
  note "ALSA dev headers missing — 'sudo apt-get install libasound2-dev' to build with audio"
fi

# --- Audio runtime -----------------------------------------------------------
printf '\nAudio runtime\n'
if [ -n "${PULSE_SERVER:-}" ]; then
  ok "PULSE_SERVER=$PULSE_SERVER"
elif [ -S /mnt/wslg/PulseServer ]; then
  ok "WSLg PulseAudio socket present (/mnt/wslg/PulseServer)"
else
  note "No PulseAudio server detected — sound may be unavailable; the silent build still works"
fi
routes_pulse=0
if { [ -f "$HOME/.asoundrc" ] && grep -q 'type pulse' "$HOME/.asoundrc" 2>/dev/null; } ||
   { [ -f /etc/asound.conf ] && grep -q 'type pulse' /etc/asound.conf 2>/dev/null; }; then
  ok "ALSA default routed → PulseAudio (asoundrc)"
  routes_pulse=1
else
  note "ALSA default not routed to PulseAudio — see docs/troubleshooting.md (WSL2)"
fi
# If routing through PulseAudio, the ALSA pulse plugin must be installed.
if [ "$routes_pulse" = "1" ]; then
  plugin_found=0
  for d in ${ALSA_PLUGIN_DIR:-} /usr/lib/*/alsa-lib /usr/lib/alsa-lib; do
    [ -n "$d" ] && [ -e "$d/libasound_module_pcm_pulse.so" ] && plugin_found=1
  done
  if [ "$plugin_found" = "1" ]; then
    ok "ALSA PulseAudio plugin installed"
  else
    bad "ALSA PulseAudio plugin missing — run: sudo apt-get install libasound2-plugins"
  fi
fi

# --- Live check: can the speaker actually open? ------------------------------
printf '\nLive audio check\n'
log="${TMPDIR:-/tmp}/audiopulse-doctor.log"
if [ "$have_alsa_dev" = "1" ] && command -v go >/dev/null 2>&1; then
  if go test ./internal/player/ -run TestSpeakerInitLive -count=1 >"$log" 2>&1; then
    ok "Speaker opened successfully — audio output works"
  else
    bad "Speaker could not be opened — see $log and docs/troubleshooting.md"
  fi
else
  note "Skipped (needs ALSA dev headers to build the audio backend)"
fi

# --- Spotify (optional, for full-song playback) ------------------------------
printf '\nSpotify (optional — full-song playback)\n'
if command -v librespot >/dev/null 2>&1 || [ -x "$HOME/.cargo/bin/librespot" ]; then
  ok "librespot installed"
else
  note "librespot not installed — run 'make librespot' for full-song Spotify playback"
fi
cfgdir="${XDG_CONFIG_HOME:-$HOME/.config}/audiopulse"
if [ -n "${SPOTIFY_CLIENT_ID:-}" ] || grep -qs client_id "$cfgdir/config.json" 2>/dev/null; then
  ok "Spotify Client ID configured"
else
  note "No Spotify Client ID — set SPOTIFY_CLIENT_ID or $cfgdir/config.json (see docs/getting-started.md); Deezer guest mode works without it"
fi
if [ -f "$cfgdir/token.json" ]; then
  ok "Spotify token cached (signed in)"
else
  note "Not signed in to Spotify yet — first run opens a browser"
fi

# --- Library export (optional, spotDL) ---------------------------------------
printf '\nLibrary export (optional — download to local files)\n'
if command -v spotdl >/dev/null 2>&1; then
  ok "spotdl installed"
else
  note "spotdl not installed — run 'make spotdl' to export your library to local audio"
fi
if command -v ffmpeg >/dev/null 2>&1; then
  ok "ffmpeg installed (audio conversion)"
else
  note "ffmpeg not found — spotDL needs it to convert audio (install ffmpeg, or 'spotdl --download-ffmpeg')"
fi

# --- AI assistant (optional, local Ollama/Gemma) -----------------------------
printf '\nAI assistant (optional — natural-language control, press ":")\n'
ollama_url="${AUDIOPULSE_OLLAMA_URL:-http://localhost:11434}"
if command -v ollama >/dev/null 2>&1; then
  ok "ollama installed"
else
  note "ollama not installed — the ':' assistant needs it (https://ollama.com); everything else works without it"
fi
tags="$(curl -fsS --max-time 2 "$ollama_url/api/tags" 2>/dev/null || true)"
if [ -n "$tags" ]; then
  ok "Ollama reachable at $ollama_url"
  if printf '%s' "$tags" | grep -qi 'gemma'; then
    ok "A gemma model is installed (auto-detected by the assistant)"
  elif printf '%s' "$tags" | grep -q '"name"'; then
    note "No gemma model found — run 'ollama pull gemma3', or set ollama_model in config.json"
  else
    note "No Ollama models installed — run 'ollama pull gemma3'"
  fi
  if printf '%s' "$tags" | grep -qi 'embed'; then
    ok "An embedding model is installed (library recommendations)"
  else
    note "No embedding model — run 'ollama pull nomic-embed-text' for library recommendations"
  fi
else
  note "Ollama not running — start it with 'ollama serve' to use the ':' assistant"
fi

# --- Voice control (optional, offline Vosk) ----------------------------------
printf '\nVoice control (optional — speak commands, press "v")\n'
if command -v ffmpeg >/dev/null 2>&1; then
  ok "ffmpeg present (microphone capture)"
else
  note "ffmpeg not found — voice capture needs it"
fi
if [ -f third_party/vosk/libvosk.so ] && [ -d third_party/vosk/model ]; then
  ok "Vosk library + model installed (build with 'make voice')"
else
  note "Vosk not set up — run 'make voice' to download the lib+model and enable voice"
fi
# Is a capture source available (and not obviously muted)?
src="$(pactl get-default-source 2>/dev/null || pactl info 2>/dev/null | sed -n 's/^Default Source: //p')"
if [ -n "$src" ]; then
  ok "Default capture source: $src"
  if pactl list sources 2>/dev/null | grep -A20 "Name: $src" | grep -qi 'Mute: yes'; then
    note "  …but it's muted — unmute the mic, or pick another 'voice_source' in config.json"
  fi
else
  note "No PulseAudio capture source — check your mic (WSL: WSLg forwards it as RDPSource)"
fi

# --- Summary -----------------------------------------------------------------
printf '\n\033[1mSummary:\033[0m %d ok, %d warning(s), %d failure(s)\n\n' "$pass" "$warn" "$fail"
[ "$fail" -eq 0 ]
