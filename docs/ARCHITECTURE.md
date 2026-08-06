# Architecture

`dupdig` is a single Go module. It has one binary entry point and two internal
packages: a scanning core with no external dependencies and a bubbletea TUI.

## Layout

```
main.go              entry point: version flag, mode flags, TTY detection, dispatch
cli.go               plain (non-interactive) mode, mirrors the original output
internal/scan        scan core: walk + SHA-256 hashing + report files
internal/tui         bubbletea/Lip Gloss terminal UI
```

## Dispatch (`main.go`)

`main` resolves UI mode:

1. `-v` / `--version` prints the version and exits.
2. `parseArgs` strips leading `--tui` / `--cli` flags, falling back to `modeAuto`.
3. The single positional argument is the `SourceDir`. The output directory is
   fixed to `./output` (see the `outputDirName` constant) and created in the
   current working directory.
4. In auto mode the TUI is chosen when `os.Stdout` is a terminal: `os.Stdout`
   `Stat()` reports `os.ModeCharDevice`. Piped, `nohup` and Docker invocations
   therefore always take the plain path.
5. The TUI mode ignores the returned exit code (it is interactive); the plain
   mode returns an exit code from `runCLI`.

Exit codes (plain mode): `0` success, `1` scan failure (e.g. unwritable output
dir), `2` source tree walk failure. `ScanError` carries a `Walk` flag that
maps to `2`.

## Scan core (`internal/scan`)

The package deliberately has **no external dependencies**. Everything the TUI
and CLI need is exposed as data:

- `Options` — `SourceDir`, `OutputDir`, optional `Ctx`, `Log` and `Progress`.
- `Progress` — per-file status (`Hashed`, `BytesHashed`, `CurrentPath`,
  `LastError`, `ErrorCount`, `Elapsed`) delivered after each hashed entry.
- `Result` — the finished outcome: `Files`, `DupGroups`, `EmptyFiles`,
  `EmptyDirs`, `Errors` and a `Stats` summary.
- `ScanError` — wraps failures; `Walk` distinguishes walk failures.

`Run(Options)`:

1. `MkdirAll` the output directory and create the six report files.
2. `filepath.Walk` the source tree, skipping `.git`/`.config`/`.cache`/`.local`
   and the `/dev`, `/proc`, `/run`, `/sys` roots. Symlinks are skipped.
3. Hash every regular file with SHA-256 **sequentially** by design — parallel
   reads thrash the heads of spinning disks (see the FAQ).
4. Emit `Progress` after each file. The walk is cancelable via `Ctx`.
5. Detect duplicates (same hash), empty files and empty directories.
6. Write `duplicates.txt`, `files.txt`, `empty-files.txt`, `empty-dirs.txt`,
   `errors.txt` and `rm-duplicates.sh`.

The output is byte-compatible with the pre-refactor `main.go` so existing
pipelines are unaffected.

## TUI (`internal/tui`)

A single bubbletea `model` drives everything. One scan runs on a background
goroutine; events cross back over two buffered channels:

- `progCh` — `Progress` ticks, drained by `progressCmd` (with a `select-`
  default so a slow UI never blocks the walker).
- `doneCh` — a final `scanMsg{res, err}`.

State machine in `View`:

- scanning without a result → `scanView` (animated progress)
- result/error after scan → `resultsView` / `errorView`

Rendering is split into `results.go` (full-screen layout, tabs, list rows),
`scan.go` (progress and error screens), `styles.go` (colours and text styles)
and `helpers.go` (number/duration formatting). The whole frame is padded to
the exact window size by `fillScreen`.

Navigation state lives on the model: `tab`, `cursor`, `top` (scroll offset)
and `expanded` (the set of expanded duplicate groups). Search filtering is
cached as filtered index slices (`idxDup`, `idxF`, …) rebuilt by
`ensureFiltered` only when the query or result changes.

## Dependencies

The go.mod requires only:

- `github.com/charmbracelet/bubbletea` — event loop, key input, alt screen.
- `github.com/charmbracelet/lipgloss` — text styling.

Everything else is the standard library.