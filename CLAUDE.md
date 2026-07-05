# vetpkg

A standalone `vetpkg` binary you run instead of `makepkg` (or in place of
your AUR helper's build step). It diffs the incoming `PKGBUILD` against
the last version the user approved, runs static checks, and optionally
asks an LLM (local or API) to flag suspicious changes — then, on
approval, execs the real `makepkg` and the build proceeds. `vetpkg` never
replaces, shadows, or renames the system `makepkg` binary; it's an
explicit, separate command the user opts into.

## Why this exists

AUR packages are community-maintained shell scripts (`PKGBUILD`) that run
with full user privileges via `makepkg`. Recent AUR supply-chain incidents
hijacked orphaned packages and injected infostealer payloads via modified
`PKGBUILD`s. `vetpkg` adds a review gate before build execution — vet
first, build second.

## Core design

1. **Invocation point:** a separate `vetpkg` binary the user runs
   explicitly instead of `makepkg` (e.g. `cd pkg-dir && vetpkg -si`). It
   never shadows, renames, or replaces the real `makepkg` on `$PATH` — no
   PATH-ordering tricks, no risk of clobbering the system binary. This
   means it isn't automatically transparent to yay/paru; the user (or
   their AUR-helper config) has to call `vetpkg` where they'd otherwise
   call `makepkg`.
2. **Approval cache:** `~/.cache/vetpkg/<pkgname>.sha256` (or full text)
   stores the last PKGBUILD the user said "yes" to. Unchanged packages
   skip straight through with zero friction.
3. **Static checks first:** cheap, deterministic, no network/API cost.
   Catches `curl|bash` pipes, base64-decode-then-eval, unexpected
   `source=()` domains, new npm/bun postinstall hooks, suspicious size
   deltas.
4. **LLM check second:** only runs if static checks are inconclusive or as
   a required second opinion. Sends PKGBUILD + diff, expects strict JSON
   back: `{"risk": "low|medium|high", "reasons": [...]}`.
5. **Multi-backend by interface:** `Analyzer` interface with swappable
   implementations (Claude API, Ollama local model, OpenAI, etc). A
   `MultiAnalyzer` can run several and escalate to the highest reported
   risk — never average risk down.
6. **Exit code is the enforcement mechanism:** if the user declines, `vetpkg`
   exits non-zero instead of execing `makepkg`, so no build happens. If the
   user approves, `vetpkg` resolves `makepkg` from `$PATH` (or a configured
   override) and execs it with the same args it was called with, so the
   build proceeds exactly as if `makepkg` had been invoked directly.

## Project structure

```
vetpkg/
├── CLAUDE.md               # this file
├── go.mod
├── main.go                 # entrypoint: parses args, orchestrates the flow
├── internal/
│   ├── cache/
│   │   └── cache.go        # read/write approved-version cache
│   ├── staticcheck/
│   │   └── staticcheck.go  # regex/pattern based checks, no network
│   ├── analyzer/
│   │   ├── analyzer.go     # Analyzer interface + shared types
│   │   ├── claude.go       # Claude API implementation
│   │   ├── ollama.go       # local Ollama implementation
│   │   └── multi.go        # runs multiple backends, escalates risk
│   ├── diff/
│   │   └── diff.go         # simple line diff between cached vs new PKGBUILD
│   └── config/
│       └── config.go       # loads ~/.config/vetpkg/config.json
├── config.example.json
└── install.sh              # builds binary, installs it as `vetpkg`, sets up PATH
```

## Config file (`~/.config/vetpkg/config.json`)

JSON format — no external dependencies needed.

```json
{
  "analyzer": { "backend": "claude" },
  "claude": { "model": "claude-sonnet-4-6" },
  "ollama": { "endpoint": "http://localhost:11434", "model": "llama3.1" },
  "general": { "makepkg_path": "", "auto_approve_low_risk": false }
}
```

`makepkg_path` is optional — leave it blank to resolve `makepkg` from
`$PATH` at run time (safe, since `vetpkg` is a distinctly named binary and
can't resolve back to itself). Set it only if the real `makepkg` isn't
on `$PATH` or you want to pin an exact location.

ANTHROPIC_API_KEY env var overrides the api_key config field.

## Build & run

```bash
go build -o vetpkg ./main.go
./install.sh     # builds and installs `vetpkg` to ~/.local/bin, writes default config
```

Usage: from inside an AUR package directory (where `PKGBUILD` lives), run
`vetpkg` with whatever args you'd normally pass to `makepkg` (e.g.
`vetpkg -si`). It is not a drop-in replacement invoked automatically by
yay/paru — point your AUR helper at `vetpkg` explicitly if you want it
in that path (e.g. build manually via `vetpkg` after `yay -G <pkg>`).

## Conventions for this codebase

- No LLM framework dependency (no langchaingo). Plain `net/http` +
  `encoding/json` for API calls — keeps prompt/response fully visible and
  debuggable.
- Every `Analyzer` implementation lives in its own file under
  `internal/analyzer/` and only needs to satisfy the `Analyzer` interface.
- Static checks always run before any LLM call, never after.
- Never auto-approve on `high` risk regardless of config.
- `vetpkg` must never be installed under the name `makepkg` or placed
  ahead of the real `makepkg` on `$PATH` — it is a separate, explicitly
  invoked command, not a shadow binary.

## Current status / next steps

- [x] `internal/config` — load JSON config
- [x] `internal/cache` — approve/check cached PKGBUILD hash
- [x] `internal/diff` — basic unified diff between two strings
- [x] `internal/staticcheck` — pattern rules
- [x] `internal/analyzer` — interface + Claude backend + Ollama backend
- [x] `main.go` — wire it all together, handle user confirmation prompt
- [x] `install.sh` — build binary + PATH setup (as `vetpkg`, not `makepkg`)
