# third_party/vosk

This directory holds the [Vosk](https://alphacephei.com/vosk/) native library and
speech model used by AudioPulse's offline **voice control** (`v` key). Its
contents are **downloaded, not committed** — run:

```sh
make voice        # fetches these, then builds with -tags vosk
```

After `make voice` you'll have:

| Path          | What it is                                              |
| ------------- | ------------------------------------------------------ |
| `libvosk.so`  | Vosk native library (linux-x86_64, linked via CGo)     |
| `vosk_api.h`  | Vosk C header                                           |
| `model/`      | small English acoustic model (`vosk-model-small-en-us`) |

The build links against `libvosk.so` here with an embedded rpath, so no
`LD_LIBRARY_PATH` is needed at runtime. To use a different/larger model, replace
`model/` (or set `voice_model` in `~/.config/audiopulse/config.json`). See
[ADR-0014](../../docs/adr/0014-voice-control-vosk.md).
