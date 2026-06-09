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

# --- Summary -----------------------------------------------------------------
printf '\n\033[1mSummary:\033[0m %d ok, %d warning(s), %d failure(s)\n\n' "$pass" "$warn" "$fail"
[ "$fail" -eq 0 ]
