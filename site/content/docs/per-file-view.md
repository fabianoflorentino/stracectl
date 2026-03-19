---
title: "Per-file view (Top Files)"
description: "Top opened file paths observed during a trace (TUI overlay + sidecar API)."
weight: 6
---

`stracectl` can surface the most-opened file paths observed during a trace. This
helps find hot files, repeated `ENOENT` probes, or unexpected filesystem
activity.

TUI
---

- Press `f` to open the Top Files overlay in the TUI. The panel shows the most
  opened paths and their counts (scroll with ↑/↓ or `j`/`k`).

Sidecar API
-----------

- `GET /api/files?limit=N` returns a JSON array of `{path,count}` sorted by
  descending count. Example:

```bash
curl -s 'http://localhost:8080/api/files?limit=20' | jq .
```

HTML report
-----------

- Include the top files table in the exported HTML report with `--report-top-files N`.

How it works
------------

The implementation uses a cheap heuristic that extracts pathname-like arguments
from `open`/`openat`/`creat` syscall arguments and maintains a bounded map of
path→count. To avoid unbounded memory usage, the aggregator caps the number of
distinct tracked paths and truncates overly long values.

For design details, tests, and implementation notes see `docs/per_file_view.md`.

Try it
------

Run a traced command and query the sidecar:

```bash
sudo ./stracectl run --serve :8080 curl -s https://example.com
curl -s http://localhost:8080/api/files?limit=20 | jq .
```
