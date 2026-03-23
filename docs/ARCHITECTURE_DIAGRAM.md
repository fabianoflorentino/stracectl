# stracectl Architecture Diagram (Mermaid)

```mermaid
graph LR
  subgraph CLI
    CMD["CLI\n(cmd/ - Cobra)"]
  end

  subgraph Tracing
    TRACER["Tracer\n(eBPF or strace subprocess)"]
    DISCOVER["Discover PID\n(internal/discover)"]
  end

  subgraph Pipeline
    PARSER["Parser\n(lines → SyscallEvent)"]
    AGG["Aggregator\n(in-memory stats, histograms, top-files)"]
  end

  subgraph Outputs
    TUI["TUI\n(BubbleTea)"]
    SERVER["Sidecar Server\n(HTTP / WebSocket / Prometheus)"]
    REPORT["Report Generator\n(HTML export)"]
  end

  CMD --> TRACER
  CMD --> REPLAY["Replay Mode\n(strace log)"]
  REPLAY --> PARSER

  DISCOVER --> TRACER

  TRACER --> PARSER
  PARSER --> AGG

  AGG --> TUI
  AGG --> SERVER
  AGG --> REPORT

  subgraph ServerDetails
    SERVER --> WS["WebSocket / /stream"]
    SERVER --> METRICS["Prometheus /metrics"]
    SERVER --> DEBUG["/debug/pprof, /debug/goroutines"]
    WS --> WSCLIENTS["Connected Clients"]
  end

  %% Backpressure and resilience
  TRACER --> BP[(Bounded channels & backpressure)]
  PARSER --> BP
  BP --> AGG
  AGG --> DROP[(Drop / sampling policy when overloaded)]

  %% Deployment
  subgraph Deployment
    HELM[Helm Chart / K8s Sidecar]
    HELM --> SERVER
    HELM --> TUI
  end

  %% Security
  SERVER -.-> AUTH[WS Token / bind localhost / proxy auth]
  TRACER -.-> EBPF_PRIV[Requires privileges / capabilities]

  style CMD fill:#f9f,stroke:#333,stroke-width:1px
  style TRACER fill:#ffe6a7,stroke:#333,color:#000000
  style PARSER fill:#e6f7ff,stroke:#333,color:#000000
  style AGG fill:#e6ffe6,stroke:#333,color:#000000
  style SERVER fill:#f0e6ff,stroke:#333,color:#000000
  style TUI fill:#fff0f0,stroke:#333,color:#000000
  style REPORT fill:#f7f7e6,stroke:#333,color:#000000

```

This diagram maps the primary runtime components: tracer → parser → aggregator → outputs (TUI, Server, Report), plus CLI, replay, discovery, backpressure, deployment and security considerations.

File: ARCHITECTURE_DIAGRAM.md
