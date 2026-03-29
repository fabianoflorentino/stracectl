---
title: "Privacy"
description: "Privacy-first tracing, redaction, TTL, and audit guidance for safe operation."
weight: 4
---

## Privacy-first tracing

Stracectl treats privacy as a core feature: by default the tool minimizes the risk of exposing sensitive data by applying safe defaults and configurable controls.

### Key capabilities

- Redaction of sensitive argument content and common patterns (JWTs, emails, tokens, IPs)
- Optional `--privacy-log <path>` to write newline-delimited JSON events (redacted)
- `--privacy-ttl <duration>` to automatically expire ephemeral logs (e.g., `24h`, `15m`)
- Audit file `<path>.audit` is written next to privacy logs and records `trace_start`/`trace_end` metadata and a SHA256 hash of the trace file
- `--no-args` to suppress argument capture entirely
- `--full` enables full payload capture only when explicitly requested (requires `--force` in non-interactive flows)

### Recommended practices

- Prefer ephemeral storage (tmpfs) or encrypted volumes for privacy logs.
- Files are created with mode `0600` by default; ensure file ownership and permissions are restricted.
- Use `--privacy-ttl` for short-lived investigations to ensure automatic removal of traces.
- Avoid `--full` on production systems unless you have explicit authorization and a secure storage policy.
- Verify the trace file integrity via the SHA256 recorded in the accompanying audit file before sharing.

### Quick examples

```bash
# write a redacted JSON privacy log that expires in 24 hours
stracectl run --privacy-log trace.json --privacy-ttl 24h --no-args curl https://example.com

# explicit full capture (dangerous) — only with authorization
stracectl run --privacy-log trace-full.json --full --force curl https://example.com
```

### Audit provenance

When `--privacy-log` writes to a file path, stracectl appends an audit entry at start and end that includes metadata such as actor, options, event counts, duration, and a SHA256 digest of the trace file. Use this for provenance and verification before sharing traces with third parties.

### Limitations

- Secure deletion: stracectl performs a best-effort overwrite before removal when `--privacy-ttl` expires. This is not a cryptographically secure wipe on all filesystems (e.g., copy-on-write, journaling). Prefer encrypted volumes or tmpfs for stronger guarantees.

For usage examples and operational guidance see the repo docs: `docs/privacy-usage-examples.md` and the implementation plan `docs/privacy-implementation-plan.md`.
