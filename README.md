# vetpkg

A standalone review gate for AUR builds. Run `vetpkg` instead of `makepkg`
(or point your AUR helper at it) and it will:

1. Diff the incoming `PKGBUILD` against the last version you approved.
2. Run fast, local static checks for suspicious patterns (`curl|bash`
   pipes, base64-decode-then-eval, unexpected `source=()` domains, new
   npm/bun postinstall hooks, suspicious size deltas).
3. Optionally send the PKGBUILD and diff to an LLM (Claude, a local
   Ollama model, or both) for a second opinion, returning a
   `low` / `medium` / `high` risk verdict with reasons.
4. Ask you to approve or decline the build based on the findings.
5. If approved, `exec` the real `makepkg` with the same arguments you
   passed to `vetpkg`, so the build proceeds exactly as normal. If
   declined, `vetpkg` exits non-zero and no build happens.

## Why

AUR packages are community-maintained shell scripts (`PKGBUILD`) that run
with full user privileges via `makepkg`. Recent AUR supply-chain
incidents hijacked orphaned packages and injected infostealer payloads
via modified `PKGBUILD`s. `vetpkg` adds a review step before build
execution — vet first, build second.

`vetpkg` never replaces, shadows, or renames the system `makepkg`
binary. It's a separate, explicitly invoked command — not automatically
transparent to `yay`/`paru` unless you configure them to call it.

## Install

```bash
go build -o vetpkg ./main.go
./install.sh     # builds and installs `vetpkg` to ~/.local/bin, writes default config
```

## Usage

From inside an AUR package directory (where `PKGBUILD` lives), run
`vetpkg` with whatever args you'd normally pass to `makepkg`:

```bash
cd pkg-dir
vetpkg -si
```

To use it with an AUR helper, build manually via `vetpkg` after
fetching the package (e.g. `yay -G <pkg>`), since `vetpkg` is not a
drop-in replacement invoked automatically by helpers.

## Configuration

Config lives at `~/.config/vetpkg/config.json` (see
[`config.example.json`](config.example.json)):

```json
{
  "analyzer": { "backend": "claude" },
  "claude": { "model": "claude-sonnet-4-6" },
  "ollama": { "endpoint": "http://localhost:11434", "model": "llama3.1" },
  "general": { "makepkg_path": "", "auto_approve_low_risk": false }
}
```

- `analyzer.backend` — which LLM backend to use (`claude`, `ollama`, or
  a multi-backend setup that escalates to the highest reported risk).
- `general.makepkg_path` — leave blank to resolve `makepkg` from
  `$PATH` at run time. Set it only if the real `makepkg` isn't on
  `$PATH` or you want to pin an exact location.
- `general.auto_approve_low_risk` — skip the confirmation prompt when
  both static checks and the LLM agree the risk is low. `high` risk is
  never auto-approved, regardless of this setting.

The `ANTHROPIC_API_KEY` environment variable overrides the `api_key`
config field for the Claude backend.

## Project structure

```
vetpkg/
├── main.go                 # entrypoint: parses args, orchestrates the flow
├── internal/
│   ├── cache/               # read/write approved-version cache
│   ├── staticcheck/         # regex/pattern based checks, no network
│   ├── analyzer/            # Analyzer interface + Claude/Ollama/multi backends
│   ├── diff/                # simple line diff between cached vs new PKGBUILD
│   └── config/              # loads ~/.config/vetpkg/config.json
├── config.example.json
└── install.sh               # builds binary, installs it as `vetpkg`, sets up PATH
```

## License

MIT — see [LICENSE](LICENSE).
