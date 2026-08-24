# 🧭 Kongming (孔明) — A Go Agent Framework with Three Kingdoms Theming

<div align="center">

<h3>运筹帷幄之中，决胜千里之外<br><em>Plan within the tent, win a thousand miles away</em></h3>

<p>
<strong>An open-source multi-agent orchestration framework in Go, where agents are generals of Shu Han and the orchestrator is the strategist Zhuge Liang.</strong>
</p>

<p>
<a href="https://github.com/xfengyin/kongming-agent"><img src="https://img.shields.io/badge/Go-1.21+-00ADD8?style=flat-square&logo=go" alt="Go Version"></a>
<a href="./LICENSE"><img src="https://img.shields.io/badge/License-MIT-yellow?style=flat-square" alt="License"></a>
<a href="https://github.com/xfengyin/kongming-agent/actions"><img src="https://img.shields.io/github/actions/workflow/status/xfengyin/kongming-agent/ci.yml?style=flat-square" alt="CI"></a>
</p>

<p><a href="./README.zh-CN.md">🇨🇳 中文版 README</a></p>

</div>

## Why Kongming?

Most agent frameworks are either heavyweight (LangChain/AutoGen-scale) or boring.
Kongming keeps the orchestration **lightweight and readable**, and wraps it in a
Three Kingdoms story that makes every concept memorable:

| Three Kingdoms concept | What it really is |
|---|---|
| 🏯 **军师府 Commander** | Central dispatcher: decompose task → pick generals → collect reports |
| ⚔️ **五虎将 Generals** | A pool of role-specialized agents (5 Tigers of Shu) with skills & handlers |
| 🧠 **诸葛亮 Kongming (LLM)** | An LLM-backed strategist agent that answers questions through any OpenAI-compatible API |
| 🎁 **锦囊库 Strategy Vault** | Pluggable skill/tool registry |
| 📨 **传令兵 Courier** | Message routing between components |

## ✨ Features

**Implemented & tested:**

- ✅ **Real LLM integration** — driver-based `LLMProvider` interface + OpenAI-compatible adapter (works with DeepSeek / Qwen / OpenAI out of the box)
- ✅ **隆中对 (Longzhong) demo** — `go run ./examples/longzhong` starts a real conversation with 诸葛亮 using your own API key
- ✅ **五虎将 agent pool** — 5 role agents with skill-based selection + scoring
- ✅ **Commander dispatch** — explicit general targeting (点将) or automatic skill-based selection (按技选将)
- ✅ **Courier message routing** — typed messages with delivery status
- ✅ **Prometheus metrics** — HTTP/LLM/order observability
- ✅ **Lightweight RAG (v0.2.0+)** — `pkg/knowledge` reads local `.md` files and retrieves relevant paragraphs (token-frequency matching, zero vector DB / zero external deps)
- ✅ **Core unit tests** — courier, generals, commander, LLM provider (httptest, no external network)

**On the roadmap (not yet implemented):**

- 🚧 八卦阵 workflow engine (DAG executor) — types defined, executors not wired
- 🚧 RAG / vector store — explicitly out of scope for v0.1 (see [boundaries](#boundaries))
- 🚧 Distributed scheduling / multi-tenant SaaS
- 🚧 OTel tracing (Prometheus metrics only for now)

## 🚀 Quickstart (real LLM conversation)

### Option A — `go install` (prebuilt CLI)

```bash
# Install the CLI directly from source (Go 1.21+ required)
go install github.com/xfengyin/kongming-agent/cmd/kongming@latest

# Configure any OpenAI-compatible provider
export KONGMING_API_KEY=sk-xxx
export KONGMING_BASE_URL=https://api.deepseek.com/v1   # optional, default: OpenAI
export KONGMING_MODEL=deepseek-chat                    # optional, default: gpt-4o-mini

# Start the server (metrics + health on :9090)
kongming
```

> 💡 Note: `go install` builds the `cmd/kongming` server binary. To have a
> one-shot conversation with 诸葛亮 from the command line, clone the repo and
> use Option B or download a Release artifact (Option C).

### Option B — run the 隆中对 demo from source

```bash
git clone https://github.com/xfengyin/kongming-agent.git
cd kongming-agent

# 1. Configure any OpenAI-compatible provider
export KONGMING_API_KEY=sk-xxx
export KONGMING_BASE_URL=https://api.deepseek.com/v1   # optional, default: OpenAI
export KONGMING_MODEL=deepseek-chat                    # optional, default: gpt-4o-mini

# 2. Talk to 诸葛亮
go run ./examples/longzhong/main.go
```

### Option C — download a Release binary

Prebuilt binaries (linux-amd64 / windows-amd64 / darwin-arm64) are attached to
every [GitHub Release](https://github.com/xfengyin/kongming-agent/releases)
tagged `v*`. Download and unpack, then run:

```bash
# linux / macOS
tar -xzf kongming-linux-amd64.tar.gz
cd kongming-linux-amd64 && ./kongming

# windows
# unzip kongming-windows-amd64.zip, then run kongming.exe
```

Example interaction:

```
=== 隆中对 · 孔明军师 ===
主公> 天下三分，魏蜀吴鼎立，亮以为当如何？
🧠 诸葛亮：
天下大势……（real LLM answer）
```

No API key? Try the offline demo:

```bash
go run ./examples/longzhong/main.go --mock
```

One-shot mode:

```bash
go run ./examples/longzhong/main.go --ask "如何提升团队执行力？"
```

Multi-turn mode (v0.2.0+, keeps in-memory conversation history):

```bash
go run ./examples/longzhong/main.go --interactive
# or offline: go run ./examples/longzhong/main.go --mock --interactive
```

Knowledge-base mode (v0.2.0+, lightweight RAG — retrieves relevant passages from local `.md` files and injects them into the LLM context):

```bash
# try the bundled Three Kingdoms knowledge base
go run ./examples/longzhong/main.go --knowledge ./knowledge --ask "司马懿兵临城下，如何用空城计退敌？"
# combine with multi-turn: --interactive --knowledge ./knowledge
# point at your own knowledge dir: --knowledge /path/to/your/markdown/docs
```

## 🧪 Run the tests

```bash
make test        # go test -v -race -cover ./...
make build       # build ./cmd/kongming binary
make run-example # run the structural quickstart demo
```

## 📦 Project layout

```
kongming-agent/
├── cmd/kongming/            # server entrypoint (metrics + health)
├── examples/
│   ├── longzhong/           # ⭐ real LLM conversation with 诸葛亮
│   └── quickstart/          # structural demo (no API key needed)
├── internal/memory/         # three-tier memory store
├── pkg/
│   ├── core/                # shared domain types (order/report/strategy)
│   ├── cmd_center/          # 军师府 Commander dispatcher
│   ├── generals/            # 五虎将 agent pool + 诸葛亮 LLM strategist
│   ├── llm/                 # LLMProvider interface + OpenAI-compatible adapter
│   ├── knowledge/           # lightweight RAG: local .md knowledge base (v0.2.0+)
│   ├── courier/             # 传令兵 message routing
│   ├── bagua/               # 八卦阵 workflow engine (roadmap)
│   ├── dispatch/            # async task dispatcher
│   ├── observatory/         # Prometheus metrics
│   ├── repeater/            # retry with backoff
│   └── strategy_vault/      # 锦囊库 skill registry
├── configs/                 # YAML configuration
└── deployments/             # Prometheus/Grafana compose files
```

## 🔌 LLM Provider configuration

Kongming uses a driver-based design — one interface, many providers:

| Env var | Required | Default | Description |
|---|---|---|---|
| `KONGMING_API_KEY` | ✅ | — | API key (any OpenAI-compatible service) |
| `KONGMING_BASE_URL` | — | `https://api.openai.com/v1` | Base URL, e.g. `https://api.deepseek.com/v1`, `https://dashscope.aliyuncs.com/compatible-mode/v1` |
| `KONGMING_MODEL` | — | `gpt-4o-mini` | Model name, e.g. `deepseek-chat`, `qwen-plus` |
| `KONGMING_PROVIDER` | — | `openai-compatible` | Label used in metrics |

Implement your own provider by satisfying the `llm.Provider` interface:

```go
type Provider interface {
    Name() string
    Chat(ctx context.Context, req llm.ChatRequest) (*llm.ChatResponse, error)
}
```

## 🛠️ Development

```bash
make fmt   # format
make test  # tests
make ci    # fmt + test + build
```

## 🚧 Boundaries (what we deliberately do NOT do)

1. **No self-hosted LLM / training / fine-tuning** — model capability is fully delegated to external APIs.
2. **No RAG engine / vector DB in v0.1** — if needed later, only "read a local file into context" minimal form.
3. **No visual workflow editor / distributed scheduling / multi-tenant SaaS** — v0.1 is a single-process, single-user local framework.
4. **Not a LangChain/AutoGen feature-complete clone** — it is a light, readable, Three Kingdoms-themed teaching & rapid-prototyping framework.
5. **No vendor lock-in** — no default keys, no proxy relay; missing key ⇒ the demo errors out with setup guidance.
6. **Not production-grade** — research/demo level; harden before production use. No SLA.
7. **Theming only** — Three Kingdoms naming/narrative is packaging, not a promise of role-play quality.

## 📄 License

MIT License — see [LICENSE](LICENSE).

---

<div align="center"><p><strong>「非淡泊无以明志，非宁静无以致远」</strong><br>— Zhuge Liang</p></div>
