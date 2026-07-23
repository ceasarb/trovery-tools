# Demigo Tools 🦉

### In tandem with AI.

*Pronounced **DEM-ee-go** — "Demi," your companion, ready to go.*

The Go toolchain behind **[Demigo](https://github.com/ceasarb/demigo)** — one platform for the whole
lifecycle of building *with* AI: **decide → build → govern**. This repo is the **build** and **govern**
halves; the **decide** half is the `/demi:` method that installs into Claude Code from the
[demigo](https://github.com/ceasarb/demigo) repo.

```
demi           the dispatcher — one verb over two tools
├── demi forge     build your AI    — agents, MCP servers, skills, evals, deploy
└── demi vigil     govern your AI   — sessions, policy, audit, SARIF
```

One shared SKILL.md parser keeps the two honest; two independently-releasable binaries keep them
decoupled.

---

## Install

Requires Go 1.25+.

```bash
git clone https://github.com/ceasarb/demigo-tools.git ~/Developer/demigo-tools
cd ~/Developer/demigo-tools
make install     # builds & installs: demi (dispatcher), demi-forge, demi-vigil
```

`make build` drops the binaries in `./bin/` instead. `make ci` runs vet + build + test.

---

## `demi forge` — build

Take an agent from prototype to production: scaffold it, wire MCP servers and skills, run eval suites,
and generate deploy artifacts for Docker / Kubernetes / Cloud Run / Container Apps.

```bash
demi forge init my-workspace          # scaffold a .demi/ workspace
demi forge agent create researcher    # new agent (agent.yaml)
demi forge server create fetch        # new MCP server
demi forge agent add-server researcher --server ./servers/fetch
demi forge skill create summarize     # scaffold a SKILL.md
demi forge agent eval researcher      # run eval suites
demi forge agent deploy researcher --target kubernetes
demi forge dashboard                  # local web dashboard
```

Command groups: `agent`, `server`, `skill`, `model` (local Ollama/HuggingFace), `registry`, `deploy`,
`dashboard`. Provider back-ends: Anthropic, OpenAI, Ollama.

## `demi vigil` — govern

Wrap any AI coding session (Claude Code, Codex, Cursor, or a bare command) in a governed, recorded,
policy-checked run — with filesystem/secrets/skills policy and SARIF output for CI.

```bash
demi vigil init                       # scaffold .demi/vigil.yaml + policy
demi vigil start                      # begin a governed session
demi vigil run claude                 # launch a tool inside the session
demi vigil stop
demi vigil report                     # usage report from recorded sessions
demi vigil audit --ci --format sarif  # re-check a session / git range → SARIF
demi vigil policy show                # merged org + repo policy
```

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
    ├── forge/          # forge implementation
    ├── vigil/          # vigil implementation
    └── skill/          # the one shared SKILL.md parser
```

## License

MIT. See [LICENSE](LICENSE).
