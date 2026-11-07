# CLAUDE.md

Guidance for this repository lives in AGENTS.md, shared with all AI coding
agents. Claude Code reads it via the import below.

@AGENTS.md

## Fork notes

- `serve s3` on small hosts (1 GB RAM / 1 CPU): defaults (`--multipart-streaming-buffer-limit 256M`,
  `--multipart-streaming-buffer-total 1G`) OOM with a few concurrent multipart uploads. Tested safe set:
  `GOMEMLIMIT=600MiB --multipart-streaming-buffer-limit 64M --multipart-streaming-buffer-total 192M
  --buffer-size 4M --use-mmap --metadata-max-objects 20000` (+ `--ignore-checksum` for ~50% more throughput).
- MD5 ETags of big objects on `local` survive restarts only through a `hasher` remote.
- e2e checks: run the server in `docker run --memory=1g --cpus=1`, drive it with `rclone rcat` multipart uploads.
