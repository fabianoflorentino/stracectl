# Privacy Usage Examples

This page shows recommended ways to use stracectl's privacy features safely. Keep everything in English.

## Principles

- Default behaviour is privacy-first: avoid capturing sensitive argument content unless explicitly requested.
- Prefer ephemeral storage and short TTLs for trace logs containing potentially sensitive data.
- Use the audit file (`<privacy-log>.audit`) as provenance: it records `trace_start` and `trace_end` metadata and a SHA256 hash of the trace file.

## Recommended commands

Write a redacted JSON privacy log that expires after 24 hours, do not capture argument contents:

```bash
stracectl run --privacy-log trace.json --privacy-ttl 24h --no-args curl https://example.com
```

Stream redacted events to stdout (no audit file written):

```bash
stracectl run --privacy-log stdout --no-args some-command
```

Explicitly enable full capture (dangerous):

```bash
# WARNING: this may capture tokens, credentials and PII
stracectl run --privacy-log trace-full.json --full --force some-command
```

## Audit verification

When using a file output (not `stdout`), stracectl writes an audit file next to the privacy log with a `.audit` suffix. The audit file contains JSON entries like:

```json
{
  "action": "trace_start",
  "label": "my-run",
  "privacy_opts": {"no_args": true, "ttl": "24h"},
  "ts": "2026-03-29T12:00:00Z",
  "actor": "username"
}
```

and on exit:

```json
{
  "action": "trace_end",
  "event_count": 12345,
  "file_hash": "<sha256 of trace file>",
  "duration": "1m2s",
  "ts": "2026-03-29T12:01:02Z"
}
```

The `file_hash` lets operators verify the integrity of the trace file before sharing it with a third party.

## Operational recommendations

- Use `tmpfs` or an encrypted filesystem for storing sensitive logs.
- Files are created with mode `0600`; ensure the filesystem and process owner are restricted.
- Rotate and remove logs when no longer needed — `--privacy-ttl` automates expiry for ephemeral cases.
- Avoid running `--full` on production workloads unless explicitly authorized.

## FAQ

Q: Why does `--privacy-log stdout` not create an audit file?
A: Writing to `stdout` is considered ephemeral by default and avoids creating on-disk artifacts. If you need audit provenance, write to a file and rely on the generated `.audit` file.

Q: Is the secure overwrite on TTL expiry guaranteed?
A: The tool performs a best-effort overwrite before removal. On modern copy-on-write or journaled filesystems this is not a cryptographically secure erasure. Prefer encrypted volumes or tmpfs for stronger guarantees.

---

See `docs/privacy-implementation-plan.md` for the design rationale and interface definitions.
