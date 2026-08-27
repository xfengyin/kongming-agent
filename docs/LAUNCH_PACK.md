# 发布启动包（LAUNCH PACK）— kongming-agent v0.8.0

> 配套 M9（v0.8.0 准备）。目标：6 个月 star ≥ 50（止损 gate）。
> 本文件是「发布当天就能照着执行」的操作包：最终稿、顺序、tracking、录屏。

---

## 一、发布顺序（D-Day 执行清单）

| 顺序 | 动作 | 平台 | 负责人 | 完成 |
|---|---|---|---|---|
| 1 | 打 v0.8.0 tag，确认 Release 流水线成功（三平台二进制） | GitHub | 维护者 | ☐ |
| 2 | 录屏 demo（脚本见第五节） | 本地 | 维护者 | ☐ |
| 3 | 掘金发技术向文章（最终稿见二.1） | 掘金 | 维护者 | ☐ |
| 4 | 知乎发叙事向文章（最终稿见二.2） | 知乎 | 维护者 | ☐ |
| 5 | HN Show HN（最终稿见二.3） | news.ycombinator.com | 维护者 | ☐ |
| 6 | awesome 清单投稿（见二.4） | GitHub awesome repos | 维护者 | ☐ |
| 7 | 各渠道补链接 + 记录数据（tracking 表见三） | — | 维护者 | ☐ |

> ⚠️ 顺序原则：先让 Release 可下载（1）→ 有画面可看（2）→ 再发长文（3/4）→
> 最后投聚合清单（6）。避免「文章发了但 README 里下载链接还是 404」。

## 二、最终稿

### 2.1 掘金（技术向）最终稿

**标题：2500 行，一个依赖：我用 Go 写了个「诸葛亮」LLM Agent CLI**

**开头段（可直接发布）：**

> 提起 AI Agent 框架，大多数人想到的是 LangChain、AutoGen 这类庞然大物——
> 几百 MB 依赖、层层抽象，新手根本读不完。我做了个相反的尝试：一个纯 Go 的
> LLM Agent CLI，全项目 2500 行、只有一个 YAML 依赖，`kongming --mock` 一条命令
> 就能跟诸葛亮聊起来，还内置了轻量 RAG（读本地 .md）、计算器工具、会话保存加载
> 和 JSON 输出。

**正文骨架：**

1. **为什么做**：重型框架劝退新手；想做一个「能一眼看懂」的最小 Agent。
2. **架构即减法**：重构掉 9 个从未接线的死模块，16 包瘦身到 7 包。
3. **真 LLM 实装**：OpenAI 兼容协议一个接口三家厂商；10 行实现自定义 Provider。
4. **多轮对话**：`--interactive` 的 messages 透传设计 + `--history-limit` 按轮截断。
5. **轻量 RAG**：`--knowledge` 读本地 .md、词频匹配，零向量库零依赖。
6. **工具调用**：`--tool calc` 计算器短路数学问题，非数学问题自动回落 LLM。
7. **结构化输出**：`--json` 一行一个 JSON，方便集成与自动化。
8. **快速上手**：三行命令跑通；附 `--json` 输出示例（真实贴一段）。
9. **边界与反思**：不做什么（向量库/重编排/生产级），为什么限制是优点。
10. **结尾 CTA**：star / 提 issue / 一起把叙事做得更好。

**发布参数**：标签「后端 / AI」；配 1 段终端输出截图（可用 demo.md 的预期输出）。

### 2.2 知乎（叙事向）最终稿

**标题：三国 × AI：用 2500 行 Go 代码，让诸葛亮真正开口说话**

**开头段（可直接发布）：**

> 「运筹帷幄之中，决胜千里之外」——这是诸葛亮，也是我想做的那件事。
> 我写了一个 Go 的 LLM Agent CLI：你是主公，诸葛亮（LLM）回答你的每一个问题；
> 你给它一本知识库（本地 .md），它就真的会去查；你问它「计算 123*456」，
> 它就真的会算。没有 LangChain 的层层抽象，只有 7 个小包、2500 行代码，
> 每个概念都能一眼看懂。

**正文骨架：**

1. 从一句诗讲起：为什么「军师」是最合适的 Agent 人格。
2. 演示场景：与诸葛亮的一问一答 + 多轮对话 + 知识库问答 + 计算器（录屏/截图）。
3. 技术深水区：消息组装、历史透传、工具预检短路、RAG 词频注入。
4. 重构故事：从 16 包 5900 行的「展示架构」砍到 7 包 2500 行的真实产品，
   删除的每个死模块都是踩坑记录。
5. 结尾：开源地址 + 希望一起把它做成「50 star 的小而美」。

### 2.3 Hacker News（Show HN）最终稿

**Title:** `Show HN: Kongming – a lightweight LLM agent CLI themed on Three Kingdoms`

**Body:**

```
Hi HN! I built Kongming, a lightweight LLM agent CLI in Go (~2.5k LOC, one YAML
dependency). You are the lord (主公) and Zhuge Liang (诸葛亮) answers through any
OpenAI-compatible API (DeepSeek/Qwen/OpenAI).

Why? Most agent frameworks are heavyweight (LangChain/AutoGen scale) and hard to
read. Kongming keeps it light and readable — driver-based LLM interface, a
zero-dependency lightweight RAG (reads local .md files, token-frequency matching),
a calculator tool that short-circuits math questions, session save/load, and
structured JSON output for scripting.

Try it in one line (offline, no API key):
    go run github.com/xfengyin/kongming-agent/cmd/kongming@v0.8.0 --mock --ask "如何三分天下？"

Or talk to Zhuge Liang for real:
    export KONGMING_API_KEY=sk-xxx
    go run github.com/xfengyin/kongming-agent/cmd/kongming@v0.8.0 --interactive

Three Kingdoms theming is packaging, not a gimmick — the CLI is a real product
built on a tiny `pkg/agent` you can embed as a library. Docs are bilingual (EN/中文).

Happy to hear feedback on the interface design and what to build next.
```

> ⚠️ HN 版规：Show HN 需可试玩；正文里放离线可跑命令。发布后 2 小时内回复所有评论。

### 2.4 Awesome / 社区投稿

| 仓库 | 提交内容 | 入口 |
|---|---|---|
| awesome-go | Machine Learning 分类补一条 | `data/` 目录 PR |
| awesome-llm (Hannibal046) | Agent 框架段落 | README PR |
| awesome-ai-agents (e2b-dev) | 开源 Agent 框架列表 | README PR |
| 掘金「AI」标签 | 2.1 文章 | 直接发布 |
| 知乎专栏「AI 工程化」 | 2.2 文章 | 直接发布 |

投稿前自查：README 双语 ✓ / 录屏可播放 ✓ / Release 产物可下载 ✓ / CI 绿 ✓ /
License ✓ / 无敏感依赖 ✓。

## 三、Tracking 表（发布后每天更新一次）

| 日期 | 渠道 | 链接/位置 | 阅读/查看 | 点赞/star | 评论/回复 | 转化 star | 备注 |
|---|---|---|---|---|---|---|---|
| D-Day | GitHub Release | /releases/tag/v0.8.0 | — | — | — | — | 基准 |
| D-Day | 掘金 | 文章 URL | — | — | — | — | 补链接 |
| D+1 | 知乎 | 文章 URL | — | — | — | — | 补链接 |
| D+1 | HN | Show HN 链接 | — | — | — | — | 回评 |
| D+3 | awesome 清单 | PR 链接 | — | — | — | — | 等 merge |
| 每周 | GitHub 仓库 | — | — | — | — | ★ 总数 | 记入止损观察 |

> 止损 gate：发布后 6 个月 star ≥ 50；<50 则归档为作品集（见 architecture 文档）。

## 四、录屏脚本（60–90 秒）

```bash
# 准备：干净终端、大字号、深色主题；先跑通一遍
cd kongming-agent

# 场景 1（0:00–0:15）：离线一问一答
go run ./cmd/kongming --mock --ask "天下大势如何？"

# 场景 2（0:15–0:40）：多轮交互（输入两问后 exit）
go run ./cmd/kongming --mock --interactive

# 场景 3（0:40–0:60）：知识库 RAG（注意"参考知识"与消息数 3 条）
go run ./cmd/kongming --mock --knowledge ./knowledge --ask "司马懿兵临城下，如何用空城计退敌？"

# 场景 4（可选 0:60–0:75）：工具调用 + JSON 输出
go run ./cmd/kongming --mock --tool calc --ask "计算 123*456"
go run ./cmd/kongming --mock --json --ask "如何三分天下？"
```

**剪辑要点**：每场景之间 1s 黑场；RAG 场景暂停 1s 让「📚 参考知识：空城计」
与「本轮收到 3 条消息」被看清；结尾 3s 定格 GitHub 地址 + star 引导；BGM 可选三国风。

## 五、v0.8.0 亮点速查（给文章引用）

- 🆕 **架构瘦身**：16 包 5900 行 → 7 包 2500 行，删 9 个从未接线的死模块，直接依赖仅 1 个（yaml）。
- 🆕 **CLI 入口**：`cmd/kongming` 从 metrics 空壳升级为真实产品（对话/RAG/工具/会话/配置）。
- 🆕 **`--history-limit`**：多轮历史按「轮」截断，长会话不再撑爆上下文。
- 🆕 **module 路径对齐**：`github.com/xfengyin/kongming-agent`，`go install` 真正可用。
- ✅ 轻量 RAG：`--knowledge <dir>` 读本地 .md，词频匹配，零向量库零依赖。
- ✅ 工具调用：`--tool calc` 计算器短路数学问题。
- ✅ 结构化输出：`--json` 每轮一个 JSON 对象，多轮结束输出 session 汇总。
- ✅ 三平台 Release：linux-amd64 / windows-amd64 / darwin-arm64。
