# dupdig
CLI tool that walks a directory tree, hashes every file with SHA-256, and generates reports on duplicate files.

## Install

```sh
go install github.com/fffaraz/dupdig@latest
```

## Run

```sh
nohup dupdig <source_directory> <output_directory> >output.log 2>&1 &
```

### Docker

```sh
docker run --rm -v "$PWD:/data" ghcr.io/fffaraz/dupdig /data/source /data/output
```

## Output

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

## License

[GPL-3.0](LICENSE)
