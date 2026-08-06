# dupdig

CLI tool that walks a directory tree, hashes every file with SHA-256, and generates reports on duplicate files. When run from an interactive terminal it also offers a full-screen TUI for browsing the results.

See the [docs](docs/) for the [TUI user guide](docs/TUI.md), the [architecture](docs/ARCHITECTURE.md) and the [changelog](docs/CHANGELOG.md).

## Install

```sh
go install github.com/fffaraz/dupdig@latest
```

## Run

When invoked from an interactive terminal, dupdig launches a full-screen
terminal UI (TUI): it shows live scan progress and then lets you browse
duplicates, files, empty files/dirs and errors with search.

Reports are written to an `output` directory that dupdig creates in the
current working directory:

```sh
dupdig <source_directory>
```

Force a mode explicitly:

```sh
dupdig --tui <source_directory>   # interactive TUI
dupdig --cli <source_directory>   # plain log output
```

When stdout is not a TTY (piped, `nohup`, Docker) dupdig always runs in
plain mode, keeping the original behavior:

```sh
nohup dupdig --cli <source_directory> >output.log 2>&1 &
```

### TUI keys

| Key             | Action                          |
| --------------- | ------------------------------- |
| `1`–`6` / `Tab` | switch tab                      |
| `↑` / `↓`       | move through rows (scrolls)     |
| `↩`             | expand/collapse a duplicate group |
| `/`             | filter current list (Esc clear) |
| `g` / `G`       | jump to top / bottom            |
| `r`             | re-scan                         |
| `q` / `Ctrl+C`  | quit                            |

The report files are always written to the `output` directory in your current
working directory, so the plain CLI script is still available whenever needed.

Exit codes (plain mode):

| Code | Meaning                                    |
| ---- | ------------------------------------------ |
| `0`  | scan completed                             |
| `1`  | scan failed (e.g. output not writable)     |
| `2`  | failed while walking the source tree       |

### Docker

```sh
docker run --rm -v "$PWD:/data" ghcr.io/fffaraz/dupdig /data/source
```

The report files are written to `/data/output` (i.e. `$PWD/output` on the
host) because the container's working directory is `/data`.

## Output

The reports land in the `output` directory created in your current working
directory:

- `duplicates.txt` — duplicate files sorted by wasted space
- `files.txt` — all files with hashes and sizes
- `empty-files.txt` — list of empty files
- `empty-dirs.txt` — list of empty directories
- `errors.txt` — errors encountered during traversal
- `rm-duplicates.sh` — script to delete duplicates

## FAQ

**What about symlinks?**

Symlinks are skipped. Only regular files are hashed.

**What about hard links and copy-on-write files?**

Not supported. Each path is treated as a separate file.

**Why hash every file? Can't you skip files with unique sizes?**

A common optimization is to only hash files that share the same size, and then stop hashing as soon as the bytes diverge. We intentionally skip this and fully hash every file because it also serves as a storage integrity check, similar to a ZFS scrub, catching bit-rot and silent data corruption. It is recommended to keep `files.txt` in a git repo to track changes across different runs over time.

**Why SHA-256 instead of a faster non-cryptographic hash like XXH3?**

SHA-256 is the most common hash used for verifying data integrity. Using it means you can directly compare hashes against officially published values, for example when verifying Linux ISO downloads, without needing a second tool.

**Why not hash files in parallel?**

Most NAS storage is backed by spinning disks, where concurrent reads cause the drive heads to seek back and forth between files. That seek thrashing makes parallel hashing slower than reading files one at a time, so hashing is done sequentially.

**Why remove duplicates instead of replacing them with hard links?**

Hard links are not supported across different filesystems or mount points, and many tools and backup systems do not handle them correctly.

**What changed in the UI version?**

`dupdig` now launches a full-screen TUI automatically when attached to a TTY
(`--tui` to force it, `--cli` for the plain log output). The scan core was
refactored into the `internal/scan` package, which exposes the same walk and
report generation as a structured `Result` plus live `Progress` events; the
text report files are byte-identical to before. The TUI lives in `internal/tui`
and is built with [bubbletea](https://github.com/charmbracelet/bubbletea) and
[Lip Gloss](https://github.com/charmbracelet/lipgloss). See
[docs/TUI.md](docs/TUI.md) and [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md).

## License

[GPL-3.0](LICENSE)
