# TUI user guide

When `dupdig` runs from an interactive terminal it starts a full-screen TUI: it
first shows live scan progress, then a browsable, searchable view of the
results. Pass `--cli` to get the plain log output instead.

```sh
dupdig <source_directory>
dupdig --tui <source_directory>   # force TUI
dupdig --cli <source_directory>   # force plain mode
dupdig --ignore node_modules --ignore .git <source_directory>
```

The `--ignore` flag skips directories or files by name or relative path and
works in both modes. The report files are always written to an `output`
directory that is created in your current working directory.

## Scan screen

While hashing, the screen shows:

- an animated spinner and an indeterminate progress bar;
- live counters: entries hashed, bytes read, elapsed time, errors so far;
- the file currently being hashed and the most recent error (if any).

`q` quits; `Ctrl+C` cancels the scan and quits.

## Error screen

If the scan fails fatally (for example the source directory is missing), an
error screen shows the reason. Press `q` to quit.

## Results: the six tabs

After the scan, the top strip shows counts per tab. Use `1`–`6` or `Tab` to
switch.

| # | Tab          | Shows                                                   |
| - | ------------ | -------------------------------------------------------- |
| 1 | Summary      | totals, duplicate groups, reclaimable space, scan time |
| 2 | Duplicates   | duplicate groups sorted by wasted space, expandable      |
| 3 | Files        | every hashed file with size and SHA-256                  |
| 4 | Empty files  | empty files                                              |
| 5 | Empty dirs   | empty directories                                        |
| 6 | Errors       | traversal errors                                         |

### Duplicates tab

Each row is one duplicate group: wasted bytes, total size, copy count and the
shared hash. Press `↩` (Enter) on a row to expand it and list the group's
paths; press again to collapse. `rm-duplicates.sh` in the output directory
mirrors these groups for deleting.

Navigating all list tabs: `↑`/`↓` scroll, `PgUp`/`PgDn` jump a page,
`Home`/`End` jump to start/end, `g` goes to the top and `G` to the bottom.

## Search / filter

Press `/` while on any list tab to type a filter. The list filters live,
matching paths and hashes (case-insensitive). `Backspace` deletes characters,
`Esc` clears the filter and `Enter` accepts and closes the prompt.

The Summary tab is not filterable.

## Other keys

| Key            | Action                            |
| -------------- | --------------------------------- |
| `1`–`6` / `Tab`| switch tab                        |
| `↑` / `↓`      | move through rows (scrolls)       |
| `PgUp` / `PgDn`| page up / down                    |
| `Home` / `End` | top / bottom                      |
| `g` / `G`      | jump to top / bottom of the list  |
| `↩`            | expand / collapse a duplicate group |
| `/`            | filter current list               |
| `Esc`          | clear the active filter           |
| `r`            | re-scan (still writes reports)    |
| `q` / `Ctrl+C` | quit                              |

## Troubleshooting

- **Blank screen for ~5 seconds at startup.** bubbletea/Lip Gloss query the
  terminal for its background colour on launch and wait for the reply. Modern
  terminals answer immediately; most `pty`-based test harnesses never do, so
  startup pauses until a timeout. The TUI proceeds afterwards.
- **Reports are missing.** The reports live in the `output` directory created
  in your current working directory and are written even in TUI mode at the
  end of the scan — check `./output`.