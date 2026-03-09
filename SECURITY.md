# Security Policy

## Supported Versions

Only the latest release of `stracectl` receives security updates.
Older versions are not backported.

| Version  | Supported          |
| -------- | ------------------ |
| latest   | :white_check_mark: |
| < latest | :x:                |

## Reporting a Vulnerability

**Please do not open a public GitHub issue for security vulnerabilities.**

To report a vulnerability, use one of the following methods:

- **GitHub Private Security Advisory** — open a [private advisory](https://github.com/fabianoflorentino/stracectl/security/advisories/new) directly in the repository. This is the preferred method.
- **Email** — contact the maintainer directly via the email listed in the GitHub profile [@fabianoflorentino](https://github.com/fabianoflorentino).

### What to include in your report

- A clear description of the vulnerability and the potential impact.
- Steps to reproduce or a proof-of-concept.
- The version(s) affected.
- Any suggested mitigation or fix (optional).

### What to expect

| Timeline        | Action                                                  |
| --------------- | ------------------------------------------------------- |
| Within 48 hours | Acknowledgement of the report.                          |
| Within 7 days   | Initial assessment and severity classification.         |
| Within 30 days  | Fix released or a mitigation plan communicated to you.  |

If the vulnerability is accepted, a CVE will be requested and a patched release will be published along with a public disclosure in the GitHub Security Advisories tab.

If the vulnerability is declined, you will receive an explanation of why it was not considered a security issue.

## Scope

The following are considered in scope:

- Code within this repository (`cmd/`, `internal/`, `main.go`).
- The official Docker image published to `fabianoflorentino/stracectl` on Docker Hub.
- The Helm chart under `deploy/helm/stracectl`.

Dependencies (Go modules, base Docker images) that are vulnerable should be reported to their respective upstream projects. We keep dependencies up to date via [Dependabot](.github/dependabot.yml).
