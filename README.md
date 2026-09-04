# 🧭 Kongming (孔明) — A Lightweight LLM Agent CLI in Go

<div align="center">

<h3>运筹帷幄之中，决胜千里之外<br><em>Plan within the tent, win a thousand miles away</em></h3>

<p>
<strong>Talk to Zhuge Liang himself. A lightweight LLM agent CLI built on proven libraries:
conversation, local knowledge base (RAG), tool calling, and session persistence.</strong>
</p>

<p>
<a href="https://github.com/xfengyin/kongming-agent"><img src="https://img.shields.io/badge/Go-1.21+-00ADD8?style=flat-square&logo=go" alt="Go Version"></a>
<a href="./LICENSE"><img src="https://img.shields.io/badge/License-MIT-yellow?style=flat-square" alt="License"></a>
<a href="https://github.com/xfengyin/kongming-agent/actions"><img src="https://img.shields.io/github/actions/workflow/status/xfengyin/kongming-agent/ci.yml?style=flat-square" alt="CI"></a>
</p>

<p><a href="./README.zh-CN.md">🇨🇳 中文版 README</a></p>

</div>

## What it is

Kongming is a **zero-framework-dependency LLM agent CLI** in Go. One binary, one YAML dependency,
a handful of small packages — each one readable in a sitting. The Three Kingdoms story is the theme:
you are the lord (主公), and the strategist Zhuge Liang (诸葛亮) answers through any
OpenAI-compatible API (DeepSeek / Qwen / OpenAI all work through the same interface).

| Feature | What it does |
|---|---|
| 💬 **Conversation** | Single-turn `--ask` or multi-turn `--interactive` REPL |
| 📚 **Lightweight RAG** | Read local `.md` files, retrieve relevant paragraphs into context — no vector DB |
| 🛠️ **Tool calling** | Built-in calculator: recognize `计算 123*456`, evaluate safely, short-circuit |
| 🔌 **MCP tools** | Mount any stdio MCP Server (filesystem, git, web…) via `--mcp` — protocol handled by `mark3labs/mcp-go` |
| 💾 **Session persistence** | Save / load a conversation (+ knowledge config) as JSON |
| ⚙️ **Config files** | YAML or JSON, with env-var overrides |
| 🧪 **Library-usable** | `pkg/agent` exposes a clean `Agent.Ask()` API for embedding |

## ✨ Features

- ✅ **Real LLM integration** — `llm.Provider` interface + OpenAI-compatible adapter
- ✅ **Multi-turn history** — automatic history threading; optional per-round truncation (`--history-limit`)
- ✅ **Tool pre-check** — calculator short-circuits math questions without calling the LLM
- ✅ **Lightweight RAG** — `pkg/knowledge` reads local `.md`, ranks paragraphs by token-frequency matching
- ✅ **Session save / load** — atomic JSON persistence, knowledge dir included
- ✅ **Structured JSON output** — one object per turn, `session` summary on exit (pipeline-friendly)
- ✅ **Tests with `-race`** — agent / llm (httptest) / knowledge / session / tools / CLI integration

## 🚀 Quickstart

### Option A — install the CLI

```bash
# Go 1.21+ required
go install github.com/xfengyin/kongming-agent/cmd/kongming@latest

# configure any OpenAI-compatible provider
export KONGMING_API_KEY=sk-xxx
export KONGMING_BASE_URL=https://api.deepseek.com/v1   # optional, default OpenAI
export KONGMING_MODEL=deepseek-chat                    # optional, default gpt-4o-mini

# no API key? try the offline demo
kongming --mock --ask "天下大势如何？"
```

### Option B — build from source

```bash
git clone https://github.com/xfengyin/kongming-agent.git
cd kongming-agent

export KONGMING_API_KEY=sk-xxx
go run ./cmd/kongming --interactive
```

### Option C — download a Release binary

Prebuilt binaries (linux-amd64 / windows-amd64 / darwin-arm64) are attached to every
[GitHub Release](https://github.com/xfengyin/kongming-agent/releases) tagged `v*`:

```bash
tar -xzf kongming-linux-amd64.tar.gz
cd kongming-linux-amd64 && ./kongming --mock
```

## Usage

```
kongming [flags]

  --mock               offline demo, no API key required
  --ask "问题"          ask one question and exit
  --interactive        multi-turn REPL (in-memory history)
  --knowledge DIR      lightweight RAG: local .md knowledge base
  --json               structured JSON output (one object per turn)
  --save PATH          save session as JSON on exit
  --load PATH          load a session (history + knowledge) and continue
  --tool calc          enable the built-in calculator
  --mcp "npx -y @modelcontextprotocol/server-filesystem /tmp"
                        mount a stdio MCP Server as tools
  --config PATH        YAML/JSON config file (env vars take priority)
  --history-limit N    cap history to the most recent N rounds (0 = unlimited)
  --version            print version and exit
```

Examples:

```bash
# one-shot
kongming --mock --ask "如何提升团队执行力？"

# offline interactive with knowledge base and calculator
kongming --mock --interactive --knowledge ./knowledge --tool calc

# knowledge retrieval: "空城计" matches the bundled Three Kingdoms passages
kongming --mock --knowledge ./knowledge --ask "司马懿兵临城下，如何用空城计退敌？"

# JSON output (pipe to jq)
kongming --mock --json --ask "如何三分天下？"
# {"type":"turn","question":"如何三分天下？","general":"诸葛亮","answer":"...","model":"mock-model","success":true,"turns":2}

# save a conversation, load it another day
kongming --mock --interactive --save ./session.json
kongming --mock --interactive --load ./session.json

# calculator short-circuits; non-math questions fall back to the LLM
kongming --mock --tool calc --ask "计算 123*456"   # 🧮 计算结果：123*456 = 56088

# mount a filesystem MCP server, then ask it to list allowed directories
kongming --mock --mcp "npx -y @modelcontextprotocol/server-filesystem /tmp" --ask "list_allowed_directories"
```

Config file priority: **command-line flags > environment variables > config file > defaults**.
See [config.example.yaml](config.example.yaml) for every field.

## 🧪 Tests & build

```bash
make test        # go test -v -race -cover ./...
make build       # build ./cmd/kongming binary
make run-example # run the library-embedding demo
```

## 📦 Project layout

```
kongming-agent/
├── cmd/kongming/            # the CLI (flags, wiring, REPL, JSON output)
├── examples/quickstart/     # embed pkg/agent as a library demo
├── pkg/
│   ├── agent/               # ★ core orchestrator: Ask / history / tools / RAG
│   ├── config/              # YAML/JSON config loading (env overrides)
│   ├── knowledge/           # lightweight RAG: local .md knowledge base
│   ├── llm/                 # Provider interface + OpenAI-compatible adapter (go-openai) + Mock
│   ├── session/             # session persistence (atomic JSON save/load)
│   └── tools/               # tool registry + calculator (govaluate) + MCP adapter (mcp-go)
├── knowledge/               # bundled Three Kingdoms knowledge base (.md)
├── config.example.yaml      # configuration example
└── Dockerfile               # one-shot offline demo image
```

## 🔌 LLM Provider configuration

| Env var | Required | Default | Description |
|---|---|---|---|
| `KONGMING_API_KEY` | ✅ | — | API key (any OpenAI-compatible service) |
| `KONGMING_BASE_URL` | — | `https://api.openai.com/v1` | e.g. `https://api.deepseek.com/v1`, `https://dashscope.aliyuncs.com/compatible-mode/v1` |
| `KONGMING_MODEL` | — | `gpt-4o-mini` | e.g. `deepseek-chat`, `qwen-plus` |
| `KONGMING_PROVIDER` | — | `openai-compatible` | display name only |

Implement your own provider by satisfying `llm.Provider`:

```go
type Provider interface {
    Name() string
    Chat(ctx context.Context, req llm.ChatRequest) (*llm.ChatResponse, error)
}
```

## 🧩 Using it as a library

```go
import "github.com/xfengyin/kongming-agent/pkg/agent"

a := agent.New(agent.Options{
    Provider:     &llm.MockProvider{},
    SystemPrompt: agent.DefaultSystemPrompt,
    Tools:        tools.NewRegistry(tools.NewCalculator()),
    Knowledge:    kb,   // *knowledge.Store, optional
})
reply, err := a.Ask(ctx, "天下大势如何？")
```

## 🧱 Built with proven libraries

| Concern | Library | Why |
|---|---|---|
| OpenAI-compatible API | [`sashabaranov/go-openai`](https://github.com/sashabaranov/go-openai) | HTTP client, auth, retries, error mapping |
| MCP protocol | [`mark3labs/mcp-go`](https://github.com/mark3labs/mcp-go) | stdio MCP client & tool listing |
| Expression evaluation | [`Knetic/govaluate`](https://github.com/Knetic/govaluate) | safe arithmetic parser (no `eval`) |
| Config parsing | [`gopkg.in/yaml.v3`](https://pkg.go.dev/gopkg.in/yaml.v3) | YAML/JSON config files |

## 🚧 Boundaries

1. **No self-hosted LLM / training / fine-tuning** — model capability is fully delegated to external APIs.
2. **No vector DB / heavy RAG engine** — knowledge retrieval is the minimal "read local files into context" form.
3. **Single-process, single-user** — no distributed scheduling, no multi-tenant SaaS, no visual workflow editor.
4. **Not a LangChain/AutoGen clone** — deliberately light and readable; HTTP/RAG/expression details delegated to proven libraries (`go-openai`, `mcp-go`, `govaluate`, `yaml.v3`).
5. **No vendor lock-in** — no default keys; a missing key fails with setup guidance.
6. **Not production-grade** — research/demo level; harden before production use.

## 📄 License

MIT License — see [LICENSE](LICENSE).

---

<div align="center"><p><strong>「非淡泊无以明志，非宁静无以致远」</strong><br>— Zhuge Liang</p></div>
