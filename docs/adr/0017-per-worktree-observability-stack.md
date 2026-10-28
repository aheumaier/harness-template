# ADR 0017: One pinned grafana/otel-lgtm container per worktree is the observability stack

Status: proposed · 2026-10-28
Amends [ADR 0008](0008-per-worktree-isolation.md) (port block +0..+7, `reserved_ports` reads every `*_PORT`,
`.env.local` reuse)

## Context
Gap G1 of exec plan 0003 (`docs/exec-plans/*/0003-harness-gap-closure.md`, slice P1.1): the agent can read
the code but not the running app. The API logs to stdout only, its OTel provider exports nothing, and
`compose.yaml` has only `db`. [NFR-O1](../harness/nfr.md) (class O, fails, [ADR 0015](0015-anticipated-rules-and-nfr-classes-b-o.md))
asks that an agent in a fresh worktree fetches the log lines of one request by its ID, and a p95 latency,
through `make` targets only.

Two candidates, judged by the boring-tech test ([core belief 4](../design-docs/core-beliefs.md#4-prefer-boring-in-repo-reasonable-dependencies)):

- **`grafana/otel-lgtm`**: one image bundling an OTel collector, Loki (logs), Prometheus (metrics), Tempo
  (traces) and Grafana, each a widely used single-purpose upstream binary with its stock config, queried
  over its documented HTTP API (LogQL, PromQL, Tempo's trace API).
- **A bare OTel collector** writing JSON lines to `.tmp/obs/`, which `make logs`/`make metrics` read with
  `jq`. Fewer moving parts, no query language.

Plan 0003 fixed the tie-break in advance (assumption A5): take `otel-lgtm` unless, measured on the reference
machine, it needs more than 1.5 GB of RSS idle or more than 90 s to become ready.

## Measurement (2026-10-28)
- **Image:** `grafana/otel-lgtm:0.36.0@sha256:57c04cbd8e7022f32bb84c4ac005d79baf8b08e2c5be44facf11f5f1f9944ba2`
  (multi-arch index digest; pushed 2026-10-09, the newest tag at least 14 days old; 0.37.0 and later were
  too young). Bundles OTel Collector 0.162.0, Loki 3.7.8, Prometheus 3.15.0, Tempo 3.1.0, Grafana 13.2.3 and
  Pyroscope 2.3.1. 2.58 GB unpacked (arm64).
- **Machine:** Apple M4 Pro, 48 GB, macOS 26.6.2; Docker Desktop 29.6.2, Linux VM with 10 CPUs and
  7.65 GiB. Load average 3.5–6 during the runs (other work running).
- **Time to ready:** from `docker run` to the image's ready line ("The OpenTelemetry collector and the
  Grafana LGTM stack are up and running", which also creates `/tmp/ready`), image already pulled, polled
  every 100 ms: 3.46 s, then **3.70 s** (the second run, kept). At that point Grafana `/api/health`, Loki
  `/ready`, Tempo `/ready` and Prometheus `/-/ready` all answered 200.
- **Idle memory** (`docker stats`, no telemetry sent): **520 MiB** after 65 s, 531 MiB after 95 s; 3.9 %
  CPU. Ingest will raise this; the threshold is about the idle floor every worktree pays.

| | Measured | Threshold | Verdict |
|-|----------|-----------|---------|
| Idle memory | 0.53 GiB | 1.5 GB | under |
| Time to ready | 3.7 s | 90 s | under |

Assumption A5 ("~500 MB of RAM") held.

## Decision
- **The stack is one `grafana/otel-lgtm` container per worktree**, pinned as `name:tag@sha256:digest` (the
  tag says the version, the digest pins it). The pin will live in `scripts/tool-versions.env` next to
  `POSTGRES_IMAGE` (P1.4, a harness change).
- **Opt-in:** a compose profile `obs`, started by `make obs-up` (P1.4). Worktrees that don't start it pay
  nothing. Telemetry is disposable: no named volume.
- **Ports: one per signal, from the worktree's port block** ([ADR 0008](0008-per-worktree-isolation.md)).
  Blocks are 10 ports apart and use `+0..+2` today (`DB`, `API`, `WEB`); P1.4 is planned to add:

  | Offset | Container port | Use |
  |--------|----------------|-----|
  | +3 | 4318 | OTLP/HTTP ingest (logs, metrics, traces from the API, P1.3) |
  | +4 | 3100 | Loki: logs (`make logs`) |
  | +5 | 9090 | Prometheus: metrics (`make metrics`) |
  | +6 | 3200 | Tempo: traces (`make traces`) |
  | +7 | 3000 | Grafana, for a human; no make target needs it |

  OTLP/gRPC (4317) and Pyroscope (4040) stay unpublished. Every port binds to `127.0.0.1` only: OTLP is
  unauthenticated and Grafana keeps its default `admin`/`admin`. P1.4 must widen `scripts/setup-worktree.sh`
  in two places, or two worktrees can collide: the free-block probe (today `base..base+2`) and
  `reserved_ports` (today only `DB|API|WEB`).
- **Readiness:** the compose service's healthcheck tests `/tmp/ready`, so `docker compose up --wait` works as
  it does for `db`.
- **Access only through `make`:** the agent reaches the stack via `make obs-up`, `make logs REQUEST_ID=…|QUERY=…`,
  `make metrics QUERY=…` and `make traces ID=…` (P1.4). The targets source the worktree's env file
  themselves and query with `curl` + `jq`. No guide or skill tells an agent to read the env file;
  `guard-bash` blocks it.
- **Not the bare collector.** It was the fallback if a threshold failed; none did. It would also leave the
  p95 of NFR-O1 to hand-written `jq` aggregation over OTLP JSON, which PromQL's `histogram_quantile` gives
  for free from P1.3's RED histograms, and trace lookup to grepping files.
- **`jq` is a pinned download in `.bin/`** (P1.4), installed by `scripts/install-tools.sh` and verified
  against a sha256 per OS/arch like Trivy, Atlas and ShellCheck ([ADR 0016](0016-tool-supply-chain.md));
  never a system `jq`.
- **`make dev` probes the collector:** when nothing listens on `OTLP_PORT` (or the worktree has none), it
  prints "telemetry off" and sets the three `OTEL_*_EXPORTER`s to `none`. The probe only cuts noise (export
  errors every interval); the OTel config stays the only validator ([ADR 0021](0021-telemetry-export-over-otlp-http.md)).
  A `make dev` started before `make obs-up` exports nothing until restarted.
- **Who owns an image reference:** an image with a persistent volume (Postgres) is frozen per worktree in
  `.env.local` by `make setup`, so a bump never upgrades an existing data volume in place; an image without
  one (`OBS_IMAGE`) is read live from `scripts/tool-versions.env` by `make` and `setup-worktree.sh`, so a bump
  reaches every worktree at its next `make obs-up`.

## Consequences
- One 2.58 GB image per machine, shared by all worktrees; about 0.53 GiB of Docker VM memory per running
  stack at idle. On the 7.65 GiB VM measured, that bounds concurrent `obs` stacks to roughly a dozen next to
  their Postgres containers.
- **Supply chain** per [ADR 0016](0016-tool-supply-chain.md): pinned by digest, adopted only at least 14 days
  after release, and a bump is a harness PR whose provenance note compares the new digest with the tag on
  Docker Hub. The image bundles six upstream components that no harness sensor scans today; a bump PR may
  scan it with the pinned `.bin/trivy image`, never with a Trivy container image.
- Grafana, Loki, Tempo and Pyroscope are AGPL-3.0. They run as an unmodified local dev tool and never ship
  with the product.
- Telemetry is lost on `docker compose down`; that is intended for a dev loop.
- Pyroscope (profiles) runs although nothing uses it; its memory is part of the measured 0.53 GiB.
- **Supersede this ADR** when a re-measurement on a bump exceeds 1.5 GB idle or 90 s to ready; when the image
  is discontinued, re-licensed so that local dev use is no longer allowed, or loses one of the three signals; when CI must run the stack
  and its runners cannot afford it; or when NFR-O1 cannot be met on it (then the bare collector + `jq` is
  the next candidate).
