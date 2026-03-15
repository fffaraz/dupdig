# dupdig
CLI tool that walks a directory tree, hashes every file with SHA-256, and generates reports on duplicate files.

## Install

```sh
go install github.com/fffaraz/dupdig@latest
```

## Run

```sh
dupdig <source_directory> <output_directory>
```

## Output

- `duplicates.txt` — duplicate files sorted by wasted space
- `files.txt` — all files with hashes and sizes
- `empty-files.txt` — empty files
- `empty-dirs.txt` — empty directories
- `errors.txt` — errors encountered
- `rm-duplicates.sh` — script to delete duplicates
