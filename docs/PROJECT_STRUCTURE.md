# Project structure

This file contains the repository tree previously embedded in the README.

```text
.
├── bin
│   └── stracectl
├── cmd
│   ├── attach.go
│   ├── attach_test.go
│   ├── discover.go
│   ├── explain.go
│   ├── explain_test.go
│   ├── extra_test.go
│   ├── root.go
│   ├── root_test.go
│   ├── run_extra_test.go
│   ├── run_flags_test.go
│   ├── run.go
│   ├── run_select_test.go
│   ├── run_test.go
│   ├── stats.go
│   ├── stats_test.go
│   ├── trace.go
│   └── trace_test.go
├── deploy
│   ├── helm
│   │   └── stracectl
│   ├── k8s
│   │   ├── servicemonitor.yaml
│   │   └── sidecar-pod.yaml
│   ├── prometheus
│   │   ├── alerting_rules.yml
│   │   ├── prometheus.example.yml
│   │   └── recording_rules.yml
│   ├── scripts
│   │   └── apply_runner_proxy.sh
│   └── systemd
│       └── actions-runner-proxy.conf
├── docker-compose.yml
├── Dockerfile
├── docs
│   ├── actions-runner-systemd-proxy.md
│   ├── ARCHITECTURE_DIAGRAM.md
│   ├── CHANGELOG.md
│   ├── EBPF.md
│   ├── flow_attach_discover.md
│   ├── flow_ebpf_selection.md
│   ├── flow_live_tracing.md
│   ├── flow_replay.md
│   ├── flow_sidecar_server.md
│   ├── img
│   │   ├── dashboard.png
│   │   ├── detail.png
│   │   ├── log.png
│   │   └── report.jpg
│   ├── KUBERNETES.md
│   ├── LOCAL_USAGE.md
│   ├── per_file_view.md
│   ├── privacy-implementation-plan.md
│   ├── privacy-improvements.md
│   ├── privacy-usage-examples.md
│   ├── PROJECT_STRUCTURE.md
│   ├── PROMETHEUS.md
│   ├── REQUIREMENTS.md
│   ├── ROADMAP.md
│   ├── SCENARIOS.md
│   ├── SELF_HOSTED_RUNNER_LOCAL_VM.md
│   ├── SELF_HOSTED_RUNNER.md
│   ├── SELF_HOSTED_RUNNER_PROXY.md
│   ├── SYSTEM_DESIGN.md
│   ├── TESTING_CONSIDERATIONS.md
│   └── USAGE.md
├── go.mod
├── go.sum
├── internal
│   ├── aggregator
│   │   ├── aggregator.go
│   │   ├── aggregator_test.go
│   │   ├── categories.go
│   │   ├── categories_test.go
│   │   ├── errwindow.go
│   │   ├── errwindow_test.go
│   │   ├── extract_test.go
│   │   ├── extra_test.go
│   │   ├── fdmapper.go
│   │   ├── fdmapper_test.go
│   │   ├── fileattributor.go
│   │   ├── fileattributor_test.go
│   │   ├── filestats.go
│   │   ├── filestats_test.go
│   │   ├── helpers_test.go
│   │   ├── lathist.go
│   │   ├── lathist_test.go
│   │   ├── parse.go
│   │   ├── parse_test.go
│   │   ├── procinfo_test.go
│   │   ├── sorted_test.go
│   │   ├── types.go
│   │   ├── types_test.go
│   │   └── unescape_start_test.go
│   ├── discover
│   │   ├── discover.go
│   │   └── discover_test.go
│   ├── models
│   │   ├── event.go
│   │   └── event_test.go
│   ├── parser
│   │   ├── parser.go
│   │   └── parser_test.go
│   ├── privacy
│   │   ├── audit
│   │   ├── filters
│   │   ├── formatter
│   │   ├── integration_test.go
│   │   ├── output
│   │   ├── pipeline
│   │   ├── redactor
│   │   ├── types.go
│   │   └── types_test.go
│   ├── procinfo
│   │   ├── procinfo.go
│   │   └── procinfo_test.go
│   ├── report
│   │   ├── report.go
│   │   ├── report_test.go
│   │   └── static
│   ├── server
│   │   ├── server.go
│   │   ├── server_test.go
│   │   └── static
│   ├── tracer
│   │   ├── bpf
│   │   ├── ebpf_bpfeb.go
│   │   ├── ebpf_bpfeb.o
│   │   ├── ebpf_bpfel.go
│   │   ├── ebpf_bpfel.o
│   │   ├── ebpf_generate_test.go
│   │   ├── ebpf.go
│   │   ├── ebpf_helpers_test.go
│   │   ├── ebpf_stub.go
│   │   ├── errno_names.go
│   │   ├── extra_test.go
│   │   ├── run_extra_test.go
│   │   ├── select_test.go
│   │   ├── strace.go
│   │   ├── strace_test.go
│   │   ├── syscall_table.go
│   │   ├── tracer.go
│   │   └── uname.go
│   └── ui
│       ├── api
│       ├── app
│       ├── controller
│       ├── helpers
│       ├── input
│       ├── model
│       ├── overlays
│       ├── render
│       ├── styles
│       ├── syscalls.go
│       ├── syscalls_test.go
│       ├── terminal
│       ├── tui_extra_test.go
│       ├── tui.go
│       ├── tui_helpers_test.go
│       ├── tui_render_test.go
│       ├── tui_run_test.go
│       ├── tui_test.go
│       └── widgets
├── layouts
│   └── shortcodes
│       ├── features.html
│       ├── hero.html
│       └── sidecar.html
├── lefthook.yml
├── LICENSE
├── main.go
├── Makefile
├── pp.out
├── README.md
├── scripts
│   ├── cleanup-dockerhub-tags.sh
│   ├── cleanup-releases.sh
│   ├── cleanup-tags.sh
│   ├── generate-bpf.sh
│   ├── loop.sh
│   ├── run-ebpf-elevate.sh
│   └── update_changelog.sh
├── SECURITY.md
├── site
│   ├── content
│   │   ├── docs
│   │   └── _index.md
│   ├── hugo.toml
│   ├── img
│   │   └── fav.png
│   ├── layouts
│   │   └── shortcodes
│   ├── public
│   │   ├── categories
│   │   ├── css
│   │   ├── docs
│   │   ├── favicon.ico
│   │   ├── favicon.png
│   │   ├── img
│   │   ├── index.html
│   │   ├── index.xml
│   │   ├── js
│   │   ├── sitemap.xml
│   │   └── tags
│   ├── static
│   │   └── img
│   └── themes
│       └── stracectl
└── stracectl
```

## Package descriptions

The table below lists notable paths and a short English description for each package or artifact.

| Path | Description |
| --- | --- |
| [`bin/`](../bin) | Optional build artifacts and convenience binaries produced by local `go build` runs. |
| [`cmd/`](../cmd/root.go) | Cobra-based CLI entrypoints and command tests; implements `run`, `attach`, `stats`, `discover`, and related commands. |
| [`deploy/`](../deploy/helm/stracectl) | Deployment and operational assets: Helm chart, Kubernetes manifests, Prometheus rules, and helper scripts. |
| [`docker-compose.yml`](../docker-compose.yml) | Local compose configuration for integration or demo environments. |
| [`Dockerfile`](../Dockerfile) | Multi-stage container build for production images (non-eBPF and eBPF targets). |
| [`docs/`](../docs) | User and architecture documentation, diagrams, and guides. |
| [`go.mod`](../go.mod), [`go.sum`](../go.sum) | Go module configuration and dependency checksums. |
| [`internal/`](../internal) | Non-public Go packages used by the binaries (not for external import). |
| [`internal/aggregator`](../internal/aggregator/aggregator.go) | Thread-safe aggregation of syscall events, categories, counters, and sorted views. |
| [`internal/discover`](../internal/discover/discover.go) | PID and container discovery helpers used by `attach` and container workflows. |
| [`internal/models`](../internal/models/event.go) | Core domain structs (e.g., `SyscallEvent`) and related fixtures. |
| [`internal/parser`](../internal/parser/parser.go) | Parser converting raw `strace` lines or eBPF events into structured `SyscallEvent` records. |
| [`internal/privacy`](../internal/privacy/types.go) | Privacy pipeline: redaction, filters, audit metadata, and privacy-log formatting. |
| [`internal/procinfo`](../internal/procinfo/procinfo.go) | Helpers for inspecting `/proc` (fd→path mapping, cgroup/container info). |
| [`internal/report`](../internal/report/report.go) | HTML report generation using `html/template` and embedded static assets. |
| [`internal/server`](../internal/server/server.go) | HTTP sidecar server: JSON endpoints, WebSocket stream, and Prometheus metrics. |
| [`internal/tracer`](../internal/tracer/strace.go) | Tracer backends: `strace` subprocess wrapper and optional eBPF backend and artifacts. |
| [`internal/ui`](../internal/ui/tui.go) | TUI implementation (BubbleTea) and subpackages for rendering, input, controllers, and widgets. |
| [`layouts/`](../layouts/shortcodes) | Hugo site template shortcodes. |
| [`lefthook.yml`](../lefthook.yml) | Git hook configuration for pre-commit and developer tooling. |
| [`Makefile`](../Makefile) | Developer convenience targets for build, test, lint, and release. |
| [`scripts/`](../scripts) | Helper scripts for BPF generation, CI maintenance, and repository tasks. |
| [`site/`](../site) | Hugo site source used to build the project website and documentation. |
| [`stracectl`](../stracectl) | Locally-built CLI binary output (when present). |

If you prefer, I can shorten descriptions further, add extra links to package docs or primary files, or render this table as a collapsible section in the `README.md`.
