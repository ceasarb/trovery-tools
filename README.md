# Demigo Tools 🦉

### In tandem with AI.

*Pronounced **DEM-ee-go** — "Demi," your companion, ready to go.*

[![CI](https://github.com/ceasarb/demigo-tools/actions/workflows/ci.yml/badge.svg)](https://github.com/ceasarb/demigo-tools/actions/workflows/ci.yml)
[![Go Version](https://img.shields.io/github/go-mod/go-version/ceasarb/demigo-tools)](go.mod)
[![Go Report Card](https://goreportcard.com/badge/github.com/ceasarb/demigo-tools)](https://goreportcard.com/report/github.com/ceasarb/demigo-tools)
[![License: Apache 2.0](https://img.shields.io/badge/license-Apache%202.0-blue.svg)](LICENSE)

**A platform for building, serving, evaluating, and governing production AI agents.** Demigo Tools is the
infrastructure other builders stand on: scaffold an agent, wire it to MCP tools, prove it with evals,
serve it behind an HTTP API, and ship it to Docker / Kubernetes / Cloud Run / Container Apps — with
tracing, budgets, and policy enforcement wired in from the start. It's the Go toolchain behind
**[Demigo](https://github.com/ceasarb/demigo)** — one platform for the whole lifecycle of building
*with* AI: **decide → build → govern**. This repo is the **build** and **govern** halves; the **decide**
half is the `/demi:` method that installs into Claude Code from the
[demigo](https://github.com/ceasarb/demigo) repo.

```
demi           the dispatcher — one verb over two tools
├── demi forge     build your AI    — agents, MCP servers, skills, evals, serve, deploy
└── demi vigil     govern your AI   — sessions, policy, audit, SARIF
```

One shared SKILL.md parser keeps the two honest; two independently-releasable binaries keep them
decoupled.

---

## Platform capabilities

Everything below is wired and tested today — not roadmap.

| Capability | What it gives you |
|---|---|
| **Agent runtime** (plan · act · observe) | Tool-call loop with sequential *and* parallel tool execution, streaming activity sinks, per-session cost accounting |
| **Model gateway** | One `Provider` interface over Anthropic, OpenAI, and Ollama, with per-model cost accounting, retry honoring `Retry-After`, and rate limiting. Swap models without touching agent code |
| **MCP toolchain** | Scaffold → hot-reload dev REPL → protocol-compliance validator (naming, schema, security, errors, pagination, annotations, response shape) → registry → sandbox |
| **Eval stack** | 20 assertion types (tool-use, output, cost, latency, MCP response shape), baseline snapshots for regression detection, HTML reports |
| **Serving API** | `POST /invoke`, `POST /invoke/stream` (SSE), `GET /health`, `GET /ready`, `GET /metrics` — the internal "assistance API" other services call |
| **Observability** | OpenTelemetry (OTLP) traces, Prometheus metrics, health/readiness probes, a session recorder for replay |
| **Guardrails & reliability** | Per-request cost cutoffs enforced in-process, monthly budgets tracked in a local cost store (see the note under [Serving API](#serving-api--the-assistance-api) before relying on it across replicas), provider retry honoring `Retry-After` with exponential backoff, sandbox egress firewall (iptables allowlist, default-DROP) |
| **Deploy** | Generate Docker, Kubernetes, Cloud Run, Container Apps, and GitHub Actions artifacts from one agent config |
| **Governance** (`vigil`) | Wrap any AI coding session in a recorded, policy-checked run with filesystem/secrets/skills policy and SARIF output for CI |

### Architecture

```mermaid
flowchart LR
    subgraph forge["demi forge — build & serve"]
        RT["Agent runtime<br/>plan · act · observe"]
        GW["Model gateway<br/>Anthropic · OpenAI · Ollama"]
        MCP["MCP servers<br/>validate · sandbox · registry"]
        EVAL["Eval stack<br/>assertions · regression"]
        API["Serving API<br/>/invoke · SSE · /metrics"]
        RT --> GW
        RT --> MCP
        RT --> EVAL
        RT --> API
    end
    API --> OBS["Observability<br/>OTel · Prometheus"]
    API --> DEPLOY["Deploy artifacts<br/>Docker · K8s · Cloud Run · Container Apps"]
    subgraph vigil["demi vigil — govern"]
        POL["Policy engine<br/>fs · secrets · skills"]
        AUD["Audit → SARIF"]
    end
    forge -.recorded & policy-checked.-> vigil
```

---

## Install

Requires Go 1.25+.

```bash
git clone https://github.com/ceasarb/demigo-tools.git ~/Developer/demigo-tools
cd ~/Developer/demigo-tools
make install     # builds & installs: demi (dispatcher), demi-forge, demi-vigil
```

`make build` drops the binaries in `./bin/` instead. `make ci` runs vet + build + test (the same
checks CI runs).

---

## Quickstart

Clone to a served agent with cost accounting, in about a minute. Needs a provider key —
`export ANTHROPIC_API_KEY=sk-...` — or swap in `--provider ollama --model llama3.2` to stay
fully local and keyless.

**1. Scaffold a workspace and an agent.**

```bash
demi forge init quickstart && cd quickstart
demi forge agent create researcher \
  --provider anthropic --model claude-haiku-4-5 --template researcher
```

**2. Give it a tool** — scaffold a fresh MCP server and wire it in.

```bash
demi forge server create \
  --name fetch --language python --transport stdio --description "Fetch a URL"
demi forge agent add-server researcher --server ./servers/fetch
demi forge agent inspect researcher      # config, tools, and the agent's DAG
```

**3. Serve it** behind the HTTP API.

```bash
demi forge agent serve researcher --no-auth --port 8080
```

Then from another shell:

```bash
curl -s localhost:8080/health
curl -s localhost:8080/invoke -H 'content-type: application/json' \
  -d '{"message": "In one sentence, what is an MCP server?"}'
```

**4. Meter it.** Every response carries usage and cost as headers:

```bash
curl -sD - -o /dev/null localhost:8080/invoke -H 'content-type: application/json' \
  -d '{"message": "hello"}' | grep -i '^x-demi-'
```

You get `X-Demi-Tokens-In`, `X-Demi-Tokens-Out`, `X-Demi-Tool-Calls`, and `X-Demi-Cost`. Set
`budget_per_request` or `budget_monthly` in `agent.yaml` and the same call also returns
`X-Demi-Monthly-Remaining`, plus `X-Demi-Budget-Exceeded` once a cap is hit. Read the note under
[Serving API](#serving-api--the-assistance-api) before relying on `budget_monthly` across replicas.

**5. Stream, observe, ship.**

```bash
curl -N localhost:8080/invoke/stream -H 'content-type: application/json' \
  -d '{"message": "Give me two quick facts about Go."}'   # SSE: text, tool_start, tool_result, done
curl -s localhost:8080/metrics | grep '^demi_'            # Prometheus
demi forge agent deploy researcher --target kubernetes    # → deploy/kubernetes/
```

From here: [`demi forge`](#demi-forge--build) for evals, skills, and sandboxing, or
[`demi vigil`](#demi-vigil--govern) to wrap the work in a recorded, policy-checked session.

---

## `demi forge` — build

Take an agent from prototype to production: scaffold it, wire MCP servers and skills, run eval suites,
serve it behind an HTTP API, and generate deploy artifacts.

**The flow — zero to a deployed agent:**

```bash
demi forge init my-workspace                 # scaffold a .demi/ workspace
demi forge agent create researcher           # new agent → agent.yaml
demi forge server create --name fetch \
  --language python --transport stdio        # new MCP server the agent can call
demi forge agent add-server researcher \
  --server ./servers/fetch                   # wire the server into the agent
demi forge agent skill create summarize      # scaffold a SKILL.md and attach it
demi forge agent chat researcher             # talk to it locally to sanity-check
demi forge agent eval researcher             # run eval suites — prove it works
demi forge agent serve researcher            # serve it behind an HTTP API (see below)
demi forge agent deploy researcher \
  --target kubernetes                        # emit deploy artifacts (see targets below)
```

`agent deploy --target` is required; pick one of `docker`, `kubernetes`, `cloud-run`,
`container-apps`, `github-actions`, or `all`. Artifacts land in `./deploy/` by default.

### Serving API — the "assistance API"

`agent serve` puts an agent behind a small, production-shaped HTTP surface that internal services can
call. Auth is on by default (auto-generated bearer token); `--no-auth` is for local dev only.

```bash
demi forge agent serve researcher --no-auth --port 8080

# Blocking call — JSON in, JSON out
curl -s localhost:8080/invoke \
  -H 'content-type: application/json' \
  -d '{"message": "Summarize the latest release notes"}'
# → {"response":"…","usage":{"input_tokens":…,"output_tokens":…},"tool_calls":2,"cost_usd":0.0123,"model":"…"}

# Streaming call — Server-Sent Events (text, tool_start, tool_result, done)
curl -N localhost:8080/invoke/stream \
  -H 'content-type: application/json' \
  -d '{"message": "Research and draft a summary"}'
```

| Endpoint | Purpose |
|---|---|
| `POST /invoke` | Blocking invocation; returns response + usage + cost |
| `POST /invoke/stream` | SSE stream of tokens and tool-call events |
| `GET /health` | Liveness probe |
| `GET /ready` | Readiness probe |
| `GET /metrics` | Prometheus metrics |

Every response carries cost/usage headers — `X-Demi-Tokens-In`, `X-Demi-Tokens-Out`, `X-Demi-Cost`,
`X-Demi-Monthly-Remaining` — and the server returns `429` when the rate limit trips and
`X-Demi-Budget-Exceeded` when a monthly cap is hit. `--sandbox` runs bundled MCP servers in
egress-firewalled containers.

> **`budget_monthly` is per-instance, not global.** It's tracked in a SQLite cost store under
> `.demi/forge/`. On a stateless or autoscaled deploy — Cloud Run, or any multi-replica setup — that
> file resets on cold start and isn't shared between replicas, so the monthly total is per-instance
> and the cap won't hold across the fleet. Mount durable shared storage, or enforce the ceiling in
> your own caller, if you need a real global budget. `budget_per_request` is in-process and unaffected.

### Evals as a first-class gate

Evals are how you keep an agent honest across changes, not an afterthought.

```bash
demi forge agent eval researcher               # run the agent's suites
demi forge server eval fetch                   # run an MCP server's suites
```

20 assertion types span tool use (`tool_called`, `tool_sequence_includes`, `max_tool_calls`), output
(`output_contains`, `output_matches`), budgets (`max_cost_usd`, `max_tokens_used`,
`max_latency_seconds`), quality (`no_hallucinated_stats`, `cites_data_source`), and MCP response shape
(`schema`, `golden_file`, `range`). Baseline snapshots catch regressions run-over-run, and results
render to HTML reports.

### Observability

Agents emit OpenTelemetry (OTLP) traces and Prometheus metrics — request counts, tool-call latency,
token and cost counters — out of the box, with a session recorder for replay. Point `GET /metrics` at
Prometheus and the OTLP exporter at your collector; the `deploy` targets wire the annotations for you.

### Command reference

Top-level forge groups are `agent`, `server`, and `model`, plus `init` and `dashboard`:

| Group | What it does | Notable subcommands |
|---|---|---|
| `agent` | Agent lifecycle | `create`, `chat`, `inspect`, `add-server`, `skill`, `eval`, `serve`, `deploy`, `export` |
| `server` | MCP server lifecycle | `create`, `dev` (hot-reload REPL), `test`, `validate`, `sandbox`, `eval`, `registry`, `deploy` |
| `model` | Local models | `pull`, `list`, `remove` — Ollama, with `hf:org/name` refs resolved to Ollama names |
| `dashboard` | Local web UI over agents, servers, evals, and sessions | — |

`agent skill` (create / test / attach / detach / list / pack) manages SKILL.md skills;
`server registry` discovers and publishes MCP servers. Run `demi forge <group> --help` for the full list.

## `demi vigil` — govern

Wrap any AI coding session (Claude Code, Codex, Cursor, or a bare command) in a governed, recorded,
policy-checked run — with filesystem/secrets/skills policy and SARIF output for CI.

**The flow — govern a working session:**

```bash
demi vigil init                       # scaffold .demi/vigil.yaml + policy (commit these)
demi vigil start                      # open a governed, recorded session
demi vigil run claude                 # launch a tool inside it (claude / codex / cursor / any command)
demi vigil stop                       # finalize and persist the session
```

**Then inspect and enforce:**

```bash
demi vigil sessions                   # list recorded sessions
demi vigil log                        # replay the last session's activity
demi vigil report                     # usage report across recorded sessions
demi vigil audit --ci --format sarif  # re-check a session / git range → SARIF for CI
demi vigil policy show                # print the merged org + repo policy
demi vigil skills scan                # discover skills, then `skills check` them against policy
```

`audit` also takes `--base-ref` / `--head-ref` to check a git range instead of a session, and `--ci`
exits non-zero on error-level violations — drop it into a pipeline as a gate. `policy validate` catches
locked-field violations before they ship.

---

## On-disk layout

Everything Demigo writes lives under a single `.demi/` directory:

```
.demi/
├── forge.yaml          # forge workspace marker (committed)
├── vigil.yaml          # vigil governance config (committed, CODEOWNERS-friendly)
├── forge/              # forge runtime state (gitignored)
└── vigil/              # vigil sessions & state (gitignored)
```

## Repo layout

```
demigo-tools/
├── cmd/
│   ├── demi/           # the dispatcher
│   ├── demi-forge/     # build binary
│   └── demi-vigil/     # govern binary
└── internal/
    ├── forge/          # agent runtime, model gateway, MCP toolchain, evals, serving, deploy
    ├── vigil/          # sessions, policy engine, audit, SARIF reporting
    └── skill/          # the one shared SKILL.md parser
```

## License

Apache License 2.0. See [LICENSE](LICENSE).
