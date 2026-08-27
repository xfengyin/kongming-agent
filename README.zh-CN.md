# 🧭 孔明军师（Kongming）—— Go 轻量 LLM Agent CLI

<div align="center">

<h3>运筹帷幄之中，决胜千里之外</h3>

<p>
<strong>与诸葛亮对谈。一个零框架依赖的 LLM Agent CLI：
对话、本地知识库（RAG）、工具调用、会话持久化，麻雀虽小五脏俱全。</strong>
</p>

<p>
<a href="https://github.com/xfengyin/kongming-agent"><img src="https://img.shields.io/badge/Go-1.21+-00ADD8?style=flat-square&logo=go" alt="Go Version"></a>
<a href="./LICENSE"><img src="https://img.shields.io/badge/License-MIT-yellow?style=flat-square" alt="License"></a>
<a href="https://github.com/xfengyin/kongming-agent/actions"><img src="https://img.shields.io/github/actions/workflow/status/xfengyin/kongming-agent/ci.yml?style=flat-square" alt="CI"></a>
</p>

<p><a href="./README.md">English README</a></p>

</div>

## 它是什么

孔明是一个 **零第三方框架依赖的 LLM Agent CLI**：一个二进制、一个 YAML 依赖、七八个小包，
每个包都小到可以一口气读完。三国叙事是它的主题——你是主公，军师诸葛亮（LLM）通过
任意 OpenAI 兼容 API（DeepSeek / 通义 / OpenAI 同一接口全通）为你运筹帷幄。

| 能力 | 说明 |
|---|---|
| 💬 **对话** | 单轮 `--ask` 或 `--interactive` 多轮 REPL |
| 📚 **轻量 RAG** | 读取本地 .md 文件，按词频检索相关段落拼入上下文——零向量库 |
| 🛠️ **工具调用** | 内置计算器：识别「计算 123*456」安全求值并短路 |
| 💾 **会话持久化** | 会话 + 知识库配置存为 JSON，可随时加载续聊 |
| ⚙️ **配置文件** | YAML 或 JSON，环境变量优先覆盖 |
| 🧪 **库化嵌入** | `pkg/agent` 暴露干净的 `Agent.Ask()` 接口 |

## ✨ 能力清单

- ✅ **真实 LLM 接入**：`llm.Provider` 接口 + OpenAI 兼容适配器
- ✅ **多轮历史**：自动透传；可选按「轮」截断（`--history-limit`）
- ✅ **工具预检**：计算器短路数学问题，不调 LLM
- ✅ **轻量 RAG**：`pkg/knowledge` 读取本地 .md，词频 + 包含匹配排序
- ✅ **会话保存/加载**：原子 JSON 写入，知识库目录随会话保存
- ✅ **结构化 JSON 输出**：每轮一个对象，退出输出 session 汇总（可管道 jq/脚本）
- ✅ **`-race` 测试**：agent / llm（httptest）/ knowledge / session / tools / CLI 集成

## 🚀 快速开始

### 方式 A — 安装 CLI

```bash
# 需 Go 1.21+
go install github.com/xfengyin/kongming-agent/cmd/kongming@latest

# 配置任意 OpenAI 兼容服务
export KONGMING_API_KEY=sk-xxx
export KONGMING_BASE_URL=https://api.deepseek.com/v1   # 可选，默认 OpenAI
export KONGMING_MODEL=deepseek-chat                    # 可选，默认 gpt-4o-mini

# 没有 Key？先离线演示一把
kongming --mock --ask "天下大势如何？"
```

### 方式 B — 源码构建

```bash
git clone https://github.com/xfengyin/kongming-agent.git
cd kongming-agent

export KONGMING_API_KEY=sk-xxx
go run ./cmd/kongming --interactive
```

### 方式 C — 下载 Release 二进制

每次打 `v*` tag 的 [GitHub Release](https://github.com/xfengyin/kongming-agent/releases)
都附带三平台预编译产物（linux-amd64 / windows-amd64 / darwin-arm64）：

```bash
tar -xzf kongming-linux-amd64.tar.gz
cd kongming-linux-amd64 && ./kongming --mock
```

## 用法

```
kongming [flags]

  --mock               离线演示，无需 API Key
  --ask "问题"          一问一答并退出
  --interactive        多轮交互 REPL（内存历史）
  --knowledge DIR      轻量 RAG：本地 .md 知识库
  --json               结构化 JSON 输出（每轮一个对象）
  --save PATH          退出时把会话存为 JSON
  --load PATH          加载会话（历史 + 知识库配置）续聊
  --tool calc          启用内置计算器
  --config PATH        YAML/JSON 配置文件（环境变量优先）
  --history-limit N    历史按「轮」截断上限（0=不限）
  --version            打印版本并退出
```

示例：

```bash
# 一问一答
kongming --mock --ask "如何提升团队执行力？"

# 离线多轮 + 知识库 + 计算器
kongming --mock --interactive --knowledge ./knowledge --tool calc

# 知识检索：「空城计」命中内置三国典故
kongming --mock --knowledge ./knowledge --ask "司马懿兵临城下，如何用空城计退敌？"

# JSON 输出（可管道 jq）
kongming --mock --json --ask "如何三分天下？"
# {"type":"turn","question":"如何三分天下？","general":"诸葛亮","answer":"...","model":"mock-model","success":true,"turns":2}

# 保存会话，改天加载续聊
kongming --mock --interactive --save ./session.json
kongming --mock --interactive --load ./session.json

# 计算器短路数学问题；非数学问题自动回落 LLM
kongming --mock --tool calc --ask "计算 123*456"   # 🧮 计算结果：123*456 = 56088
```

配置优先级：**命令行 flag > 环境变量 > 配置文件 > 内置默认**。
字段说明见 [config.example.yaml](config.example.yaml)。

## 🧪 测试与构建

```bash
make test        # go test -v -race -cover ./...
make build       # 构建 ./cmd/kongming
make run-example # 运行库化嵌入 demo
```

## 📦 目录结构

```
kongming-agent/
├── cmd/kongming/            # CLI 入口（flag/装配/REPL/JSON 输出）
├── examples/quickstart/     # 以库方式嵌入 pkg/agent 的演示
├── pkg/
│   ├── agent/               # ★ 核心编排：Ask / 历史 / 工具 / RAG
│   ├── config/              # YAML/JSON 配置加载（环境变量覆盖）
│   ├── knowledge/           # 轻量 RAG：本地 .md 知识库
│   ├── llm/                 # Provider 接口 + OpenAI 兼容适配 + Mock
│   ├── session/             # 会话持久化（原子 JSON 保存/加载）
│   └── tools/               # 工具注册表 + 计算器
├── knowledge/               # 内置三国知识库（.md）
├── config.example.yaml      # 配置示例
└── Dockerfile               # 一次性离线 demo 镜像
```

## 🔌 LLM Provider 配置

| 环境变量 | 必填 | 默认 | 说明 |
|---|---|---|---|
| `KONGMING_API_KEY` | ✅ | — | API Key（任意 OpenAI 兼容服务） |
| `KONGMING_BASE_URL` | — | `https://api.openai.com/v1` | 例如 `https://api.deepseek.com/v1`、`https://dashscope.aliyuncs.com/compatible-mode/v1` |
| `KONGMING_MODEL` | — | `gpt-4o-mini` | 例如 `deepseek-chat`、`qwen-plus` |
| `KONGMING_PROVIDER` | — | `openai-compatible` | 仅显示用 |

自定义 Provider 只需实现：

```go
type Provider interface {
    Name() string
    Chat(ctx context.Context, req llm.ChatRequest) (*llm.ChatResponse, error)
}
```

## 🧩 作为库使用

```go
import "github.com/xfengyin/kongming-agent/pkg/agent"

a := agent.New(agent.Options{
    Provider:     &llm.MockProvider{},
    SystemPrompt: agent.DefaultSystemPrompt,
    Tools:        tools.NewRegistry(tools.NewCalculator()),
    Knowledge:    kb,   // *knowledge.Store，可选
})
reply, err := a.Ask(ctx, "天下大势如何？")
```

## 🚧 产品边界（合理限制）

1. **不做自研 LLM/训练/微调**：模型能力完全委托外部 API。
2. **不引入向量库/重 RAG 引擎**：检索只做「读本地文件塞进上下文」的最简形态。
3. **单进程、单用户**：无分布式调度、无多租户 SaaS、无可视化工作流编辑器。
4. **不与 LangChain/AutoGen 对标**：刻意轻量可读，全项目约 2500 行。
5. **无平台绑定**：无默认 Key；缺 Key 时直接报错并引导配置。
6. **不承诺生产级**：研究/演示级，生产使用需自行加固。

## 📄 许可证

MIT License —— 详见 [LICENSE](LICENSE)。

---

<div align="center"><p><strong>「非淡泊无以明志，非宁静无以致远」</strong><br>—— 诸葛亮</p></div>
