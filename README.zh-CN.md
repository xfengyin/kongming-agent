# 🧭 孔明军师（Kongming）—— 三国 × AI Agent 编排框架

<div align="center">

<h3>运筹帷幄之中，决胜千里之外</h3>

<p>
<strong>一个把「三国叙事」与「真实 LLM 能力」装进同一个 Go 编排框架的开源项目。</strong>
</p>

<p>
<a href="https://github.com/xfengyin/kongming-agent"><img src="https://img.shields.io/badge/Go-1.21+-00ADD8?style=flat-square&logo=go" alt="Go Version"></a>
<a href="./LICENSE"><img src="https://img.shields.io/badge/License-MIT-yellow?style=flat-square" alt="License"></a>
<a href="https://github.com/xfengyin/kongming-agent/actions"><img src="https://img.shields.io/github/actions/workflow/status/xfengyin/kongming-agent/ci.yml?style=flat-square" alt="CI"></a>
</p>

<p><a href="./README.md">English README</a></p>

</div>

## 为什么是孔明？

市面上的 Agent 框架，要么重如泰岳（LangChain/AutoGen），要么干瘪无味。
孔明想做的是：**编排轻到能一眼看懂，叙事美到让人想用**。

主公有令，军师府拆解；五虎将各显神通；军师诸葛亮（LLM）运筹帷幄——
每一行代码背后都有一段三国故事，每一段故事都对应一个真实的工程概念。

| 三国概念 | 工程本质 |
|---|---|
| 🏯 军师府 | 中央调度器：拆解任务 → 点将/选将 → 汇总战报 |
| ⚔️ 五虎将 | 按角色分工的 Agent 池（关羽情报、张飞数据、赵云分析、马超报告、黄忠审核） |
| 🧠 诸葛亮 | LLM 驱动的军师 Agent，通过任意 OpenAI 兼容 API 真实作答 |
| 🎁 锦囊库 | 可插拔的技能/工具注册表 |
| 📨 传令兵 | 组件间的消息路由（带投递状态） |

## ✨ 能力清单（如实标注）

**已实装并有测试保障：**

- ✅ **真实 LLM 接入**：驱动式 `LLMProvider` 接口 + OpenAI 兼容适配器（DeepSeek / 通义 / OpenAI 一个接口全通）
- ✅ **隆中对 demo**：`go run ./examples/longzhong` 用你自己的 Key 与诸葛亮真实对话
- ✅ **五虎将 Agent 池**：5 位角色将领，技能匹配 + 战绩评分选将
- ✅ **军师府调度**：支持点名将领（strategy.Generals）与按技选将两种模式
- ✅ **传令兵消息路由**：类型化消息 + 投递状态
- ✅ **Prometheus 指标**：HTTP / LLM / 任务可观测
- ✅ **轻量 RAG（v0.2.0+）**：`pkg/knowledge` 读取本地 .md 文件，按词频/包含匹配检索相关段落拼入上下文（零向量库、零外部依赖）
- ✅ **核心单元测试**：courier / generals / commander / llm provider（httptest，无需外网）

**Roadmap（尚未实装，勿被旧版 README 误导）：**

- 🚧 八卦阵工作流引擎（DAG 执行器）——类型已定义，执行器未接线
- 🚧 RAG / 向量库——v0.1 明确不做（见「边界」）
- 🚧 分布式调度 / 多租户 SaaS
- 🚧 OTel 链路追踪（当前仅有 Prometheus 指标）

## 🚀 快速开始（真实对话）

### 方式 A — `go install` 安装 CLI

```bash
# 从源码直接安装（需 Go 1.21+）
go install github.com/xfengyin/kongming-agent/cmd/kongming@latest

# 配置任意 OpenAI 兼容服务
export KONGMING_API_KEY=sk-xxx
export KONGMING_BASE_URL=https://api.deepseek.com/v1   # 可选，默认 OpenAI
export KONGMING_MODEL=deepseek-chat                    # 可选，默认 gpt-4o-mini

# 启动服务（指标 + 健康检查，端口 :9090）
kongming
```

> 💡 注意：`go install` 安装的是 `cmd/kongming` 服务端二进制。若想直接在
> 命令行与诸葛亮一问一答，请用方式 B（源码运行隆中对 demo）或方式 C
> （下载 Release 产物）。

### 方式 B — 源码运行隆中对 demo

```bash
git clone https://github.com/xfengyin/kongming-agent.git
cd kongming-agent

# 1. 配置任意 OpenAI 兼容服务
export KONGMING_API_KEY=sk-xxx
export KONGMING_BASE_URL=https://api.deepseek.com/v1   # 可选，默认 OpenAI
export KONGMING_MODEL=deepseek-chat                    # 可选，默认 gpt-4o-mini

# 2. 与诸葛亮对谈
go run ./examples/longzhong/main.go
```

### 方式 C — 下载 Release 二进制

每次打 `v*` tag 的 [GitHub Release](https://github.com/xfengyin/kongming-agent/releases)
都会附带三平台预编译产物（linux-amd64 / windows-amd64 / darwin-arm64）。
下载解压后直接运行：

```bash
# linux / macOS
tar -xzf kongming-linux-amd64.tar.gz
cd kongming-linux-amd64 && ./kongming

# windows
# 解压 kongming-windows-amd64.zip 后运行 kongming.exe
```

交互示例：

```
=== 隆中对 · 孔明军师 ===
主公> 天下三分，魏蜀吴鼎立，亮以为当如何？
🧠 诸葛亮：
（真实 LLM 回答……）
```

没有 Key？离线演示一把：

```bash
go run ./examples/longzhong/main.go --mock
```

一问一答：

```bash
go run ./examples/longzhong/main.go --ask "如何提升团队执行力？"
```

多轮交互（v0.2.0+，内存历史）：

```bash
go run ./examples/longzhong/main.go --interactive
# 离线多轮：go run ./examples/longzhong/main.go --mock --interactive
```

知识库模式（v0.2.0+，轻量 RAG——检索本地 .md 相关段落注入 LLM 上下文）：

```bash
# 使用内置三国知识库
go run ./examples/longzhong/main.go --knowledge ./knowledge --ask "司马懿兵临城下，如何用空城计退敌？"
# 可叠加多轮：--interactive --knowledge ./knowledge
# 指向你自己的知识目录：--knowledge /path/to/your/markdown/docs
```

## 🧪 测试与构建

```bash
make test        # go test -v -race -cover ./...
make build       # 构建 ./cmd/kongming
make run-example # 结构演示（无需 Key）
```

## 📦 目录结构

```
kongming-agent/
├── cmd/kongming/            # 服务入口（指标 + 健康检查）
├── examples/
│   ├── longzhong/           # ⭐ 隆中对：与诸葛亮真实对话
│   └── quickstart/          # 结构演示（无需 API Key）
├── internal/memory/         # 三层记忆存储
├── pkg/
│   ├── core/                # 共享域类型（军令/战报/战略）
│   ├── cmd_center/          # 军师府调度器
│   ├── generals/            # 五虎将池 + 诸葛亮 LLM 将领
│   ├── llm/                 # LLMProvider 接口 + OpenAI 兼容适配
│   ├── knowledge/           # 轻量 RAG：本地 .md 知识库（v0.2.0+）
│   ├── courier/             # 传令兵消息路由
│   ├── bagua/               # 八卦阵工作流引擎（roadmap）
│   ├── dispatch/            # 异步任务调度
│   ├── observatory/         # Prometheus 指标
│   ├── repeater/            # 重试与退避
│   └── strategy_vault/      # 锦囊库技能注册
├── configs/                 # YAML 配置
└── deployments/             # Prometheus/Grafana 部署
```

## 🔌 LLM Provider 配置

| 环境变量 | 必填 | 默认 | 说明 |
|---|---|---|---|
| `KONGMING_API_KEY` | ✅ | — | API Key（任意 OpenAI 兼容服务） |
| `KONGMING_BASE_URL` | — | `https://api.openai.com/v1` | 例如 `https://api.deepseek.com/v1`、`https://dashscope.aliyuncs.com/compatible-mode/v1` |
| `KONGMING_MODEL` | — | `gpt-4o-mini` | 例如 `deepseek-chat`、`qwen-plus` |
| `KONGMING_PROVIDER` | — | `openai-compatible` | 指标标签用 |

自定义 Provider 只需实现：

```go
type Provider interface {
    Name() string
    Chat(ctx context.Context, req llm.ChatRequest) (*llm.ChatResponse, error)
}
```

## 🚧 产品边界（合理限制）

1. **不做自研 LLM/RAG 引擎**：模型能力完全委托外部 API；不训练、不微调、不本地推理。
2. **v0.1 不引入向量库**：RAG 若需要，只做「读本地文件塞进上下文」的最简形态。
3. **不做重编排平台**：无可视化工作流编辑器、无分布式调度、无多租户 SaaS。
4. **不与 LangChain/AutoGen 对标完整功能**：定位是「轻量、可读、三国化」的教学与快速原型框架。
5. **不做平台绑定**：无默认 Key、无代理中转；缺 Key 时 demo 直接报错并引导配置。
6. **不承诺生产级**：研究/演示级框架，生产使用需自行加固；无 SLA。
7. **三国化仅限命名与叙事**：不承诺角色扮演质量，避免误导性宣传。

## 📄 许可证

MIT License —— 详见 [LICENSE](LICENSE)。

---

<div align="center"><p><strong>「非淡泊无以明志，非宁静无以致远」</strong><br>—— 诸葛亮</p></div>
