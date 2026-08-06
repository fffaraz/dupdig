# Changelog

All notable changes are recorded here. This project does not yet follow
[Semantic Versioning](https://semver.org); the current release number scheme
is not settled.

## Unreleased

### Added

- **Terminal UI (TUI).** `dupdig` now detects an interactive terminal and
  launches a full-screen, read-only UI built with
  [bubbletea](https://github.com/charmbracelet/bubbletea) and
  [Lip Gloss](https://github.com/charmbracelet/lipgloss). See
  [TUI.md](TUI.md) for the user guide.
- Live scan progress screen with spinner, indeterminate bar and counters
  (files, bytes, elapsed, errors), plus rescan (`r`) from within the UI.
- Six browsable result tabs — Summary, Duplicates, Files, Empty files,
  Empty dirs, Errors — with scrolling, a per-tab filter (`/`), and expandable
  duplicate groups (`Enter`).
- `--tui` / `--cli` flags to force a mode. Auto mode prefers the TUI when
  stdout is a TTY, exactly as before when piped (Docker, `nohup`, CI).
- `-v` / `--version` flag.
- `internal/scan` package: the walk + SHA-256 hashing + report generation is
  factored out of `main.go` and exposed as a structured `Result` with live
  `Progress` events. The six text reports are byte-identical to the original.
- `docs/` directory with TUI, architecture and changelog documentation.
- Unit tests for the scan core and the TUI model (`internal/*/*_test.go`).

### Changed

- `main.go` slimmed down to argument parsing, mode dispatch and TTY detection;
  the original plain-mode output now lives in `cli.go`.
- The output contract is unchanged: the six report files and `rm-duplicates.sh`
  are written to the output directory in both modes, byte-identical to the
  original implementation.

### Fixed

- The scan now supports graceful cancellation (`Ctrl+C` while scanning) via
  a `context.Context` threaded through `internal/scan`.
- `--cli` retains the historical exit codes: `0` success, `1` scan failure,
  `2` failure while walking the source tree.

### Removed

- The old monolithic scan implementation that lived in `main.go` (replaced by
  `internal/scan`).

## Before the UI refactor

The tool previously walked the tree, hashed every file and printed a log to
stdout, writing six reports into the output directory. Behaviour in
`--cli` mode is preserved.

## Copyright

Copyright (C) 2026 Faraz Fallahi <fffaraz@gmail.com> — GPL-3.0. See the
[LICENSE](../LICENSE).