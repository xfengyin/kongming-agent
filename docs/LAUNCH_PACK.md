# 发布启动包（LAUNCH PACK）— kongming-agent v0.4.0

> 配套 M5（v0.4.0 准备）。目标：6 个月 star ≥ 50（止损 gate）。
> 本文件是「发布当天就能照着执行」的操作包：最终稿、顺序、tracking、录屏。

---

## 一、发布顺序（D-Day 执行清单）

| 顺序 | 动作 | 平台 | 负责人 | 完成 |
|---|---|---|---|---|
| 1 | 打 v0.4.0 tag，确认 Release 流水线成功（三平台二进制） | GitHub | 维护者 | ☐ |
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

**标题：用三国智慧编排 AI Agent：我把「五虎将」和「诸葛亮」写成了一个 Go 框架**

**开头段（可直接发布）：**

> 提起 AI Agent 框架，大多数人想到的是 LangChain、AutoGen 这类庞然大物。
> 我做了个相反的尝试：用三国题材，把「多 Agent 编排」讲成一段能记住的故事——
> 五虎将是分工明确的小 Agent，诸葛亮是真正会思考的 LLM 军师，军师府负责拆解任务、
> 调兵遣将。项目是纯 Go 写的，轻量、可读，`go run` 一条命令就能跟诸葛亮聊起来，
> 还能挂上自己的知识库（轻量 RAG）。

**正文骨架：**

1. **为什么做**：现有框架重、黑盒、劝退新手；想做一个「能一眼看懂」的框架。
2. **架构即故事**：军师府 / 五虎将 / 八卦阵 / 锦囊库 / 传令兵 ↔ 工程概念对照。
3. **真 LLM 实装**：OpenAI 兼容协议一个接口三家厂商；10 行实现自定义 Provider。
4. **多轮对话**：`--interactive` 的 messages 透传设计，为什么不做向量记忆。
5. **轻量 RAG**：`--knowledge` 读本地 .md、词频匹配，零向量库零依赖（v0.4.0 主打）。
6. **结构化输出**：`--json` 一行一个 JSON，方便集成与自动化（v0.4.0 新增）。
7. **快速上手**：三行命令跑通；附 `--json` 输出示例（真实贴一段）。
8. **边界与反思**：不做什么（重编排/RAG 引擎/生产级），为什么限制是优点。
9. **结尾 CTA**：star / 提 issue / 一起把叙事做得更好。

**发布参数**：标签「后端 / AI」；配 1 张架构图 + 1 段终端输出截图（可用 demo.md 的预期输出）。

### 2.2 知乎（叙事向）最终稿

**标题：三国 × AI：当诸葛亮变成 LLM 军师，五虎将变成 Agent 团队**

**开头段（可直接发布）：**

> 「运筹帷幄之中，决胜千里之外」——这是诸葛亮，也是每一个 AI Agent 编排系统
> 想要做到的事：把大问题拆成小任务，派给最合适的人，最后汇总成一份漂亮的战报。
> 我于是想：为什么不干脆按三国的方式做一个 Agent 框架？主公是用户，军师府是调度器，
> 五虎将各司其职，诸葛亮则是一个真正接了 LLM 的军师——你问它任何问题，它真的会回答；
> 你再给它一本知识库，它就真的会去查。

**正文骨架：**

1. 从一句诗讲起：为什么三国叙事和 Agent 编排如此契合。
2. 概念对照表：三国角色 ↔ 工程组件。
3. 演示场景：与诸葛亮的一问一答 + 多轮对话 + 知识库问答（录屏/截图）。
4. 技术深水区：messages 透传、Provider 驱动式设计、`--json` 结构化输出、
   「为什么 v0.4.0 用词频匹配而不是向量库」。
5. 踩坑记录：修复循环依赖、失效依赖版本（jaeger）等「看着完整其实编译不过」的经历。
6. 结尾：开源地址 + 希望一起把它做成「50 star 的小而美」。

### 2.3 Hacker News（Show HN）最终稿

**Title:** `Show HN: Kongming – a Go agent framework themed on Three Kingdoms`

**Body:**

```
Hi HN! I built Kongming, a lightweight Go multi-agent orchestration framework
where the agents are the Five Tiger Generals of Shu Han and the strategist
Zhuge Liang is a real LLM-backed agent.

Why? Most agent frameworks are heavyweight (LangChain/AutoGen scale) and hard to
read. Kongming keeps it light and readable — stdlib-first, driver-based LLM
interface (works with any OpenAI-compatible API: DeepSeek/Qwen/OpenAI), plus a
zero-dependency lightweight RAG (just reads local .md files, token-frequency
matching) and structured JSON output for scripting.

Try it in one line (offline, no API key):
    go run github.com/xfengyin/kongming-agent/examples/longzhong@v0.4.0 --mock --ask "如何三分天下？"

Or talk to Zhuge Liang for real:
    export KONGMING_API_KEY=sk-xxx
    go run github.com/xfengyin/kongming-agent/examples/longzhong@v0.4.0 --interactive

Three Kingdoms theming is packaging, not a gimmick — every concept maps to a real
engineering idea (Commander = dispatcher, Generals = role agents, Bagua = workflow,
Courier = message routing). Docs are bilingual (EN/中文).

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
| D-Day | GitHub Release | /releases/tag/v0.4.0 | — | — | — | — | 基准 |
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
go run ./examples/longzhong/main.go --mock --ask "天下大势如何？"

# 场景 2（0:15–0:40）：多轮交互（输入两问后 exit）
go run ./examples/longzhong/main.go --mock --interactive

# 场景 3（0:40–0:60）：知识库 RAG（注意"检索到 N 段相关知识"与消息数 3 条）
go run ./examples/longzhong/main.go --mock --knowledge ./knowledge --ask "司马懿兵临城下，如何用空城计退敌？"

# 场景 4（可选 0:60–0:75）：JSON 输出（展示结构化结果）
go run ./examples/longzhong/main.go --mock --json --ask "如何三分天下？"
```

**剪辑要点**：每场景之间 1s 黑场；RAG 场景暂停 1s 让「检索到 1 段相关知识（空城计）」
与「本轮收到 3 条消息」被看清；结尾 3s 定格 GitHub 地址 + star 引导；BGM 可选三国风。

## 五、v0.4.0 亮点速查（给文章引用）

- 🆕 `--json`：结构化输出，每轮一个 JSON 对象（role/content/turns/retrieved_knowledge），
  多轮结束输出 session 汇总 —— 可直接 pipe 给 jq / 脚本。
- ✅ 轻量 RAG：`--knowledge <dir>` 读本地 .md，词频匹配，零向量库零依赖。
- ✅ 多轮交互：`--interactive` 内存历史透传。
- ✅ 三平台 Release：linux-amd64 / windows-amd64 / darwin-arm64。
