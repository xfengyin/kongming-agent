# 推广计划（PROMOTION）— 三国 × AI Agent

> 配套 M3（v0.2.0）推广准备。目标：发布后 6 个月 star ≥ 50（止损 gate）。
> 策略依据：t2 战略报告（kongming-agent 传播性 5 分、总分 21 分第二），
> MetaGPT/AutoGPT 已验证「叙事驱动起量」。

---

## 一、核心卖点（三国 × AI）

| 卖点 | 一句话 |
|---|---|
| **题材即记忆点** | 「五虎将 = 分工 Agent，诸葛亮 = LLM 军师」——三国叙事让每个概念过目不忘 |
| **轻量可读** | 纯 Go、标准库优先，`go run ./examples/longzhong` 即可与诸葛亮真实对话 |
| **真 LLM 实装** | OpenAI 兼容协议一个接口通吃 DeepSeek / 通义 / OpenAI |
| **驱动式设计** | `LLMProvider` 接口 10 行实现即接入新厂商，无 LangChain 式重框架 |
| **多轮记忆** | v0.2.0 起支持 `--interactive` 多轮对话，历史 messages 原样透传 |
| **教学友好** | 概念骨架完整（军师府/八卦阵/锦囊库/传令兵），适合讲 Agent 编排原理 |

## 二、目标人群

1. **Go 开发者 / Agent 框架尝鲜者**（主力）：关注「可读、轻量」的实现，反感黑盒重框架。
2. **三国/历史题材爱好者**：被叙事吸引，愿意 star 支持「有趣」的项目。
3. **AI 学习者**：用它入门「多 Agent 编排 / Tool Use / 多轮对话」的基本形态。
4. **中文技术社区读者**（掘金/知乎）：对「三国 × AI」这种本土化叙事有天然亲切感。

## 三、Awesome / 社区投稿清单

- [awesome-go](https://github.com/avelino/awesome-go) — 提交到 Machine Learning 分类（子类 Agent/LLM）
- [awesome-llm](https://github.com/Hannibal046/Awesome-LLM) — Agent 框架段落
- [awesome-ai-agents](https://github.com/e2b-dev/awesome-ai-agents) — 开源 Agent 框架列表
- [awesome-golang-algorithms / awesome-go-storage] 等按实际类别投
- 掘金（juejin.cn）「后端 / AI」标签投稿
- 知乎专栏「AI 工程化 / 开源项目」
- V2EX 分享节点（注意按版规，避免纯推广）
- Hacker News（Show HN，英文标题："Show HN: Kongming – a Go agent framework themed on Three Kingdoms"）

> ⚠️ 投稿前提：README 双语完善、demo 录屏就位、v0.2.0 发布后可一键体验。

## 四、推广文草稿

### 草稿 1：掘金（技术向）

**标题：用三国智慧编排 AI Agent：我把「五虎将」和「诸葛亮」写成了一个 Go 框架**

> 摘要段（可作开头）：
> 提起 AI Agent 框架，大多数人想到的是 LangChain、AutoGen 这类庞然大物。
> 我做了个相反的尝试：用三国题材，把「多 Agent 编排」讲成一段能记住的故事——
> 五虎将是分工明确的小 Agent，诸葛亮是真正会思考的 LLM 军师，军师府负责拆解任务、
> 调兵遣将。项目是纯 Go 写的，轻量、可读，`go run` 一条命令就能跟诸葛亮聊起来。

正文结构建议：
1. 为什么做：现有框架重、黑盒、劝退新手；想做一个「能一眼看懂」的框架。
2. 架构即故事：军师府/五虎将/八卦阵/锦囊库/传令兵分别对应什么工程概念（配架构图）。
3. 真 LLM 实装：OpenAI 兼容协议一个接口三家厂商；10 行实现自定义 Provider。
4. 多轮对话（v0.2.0）：`--interactive` 的 messages 透传设计，为什么不做向量记忆。
5. 快速上手：三行命令跑通隆中对 demo。
6. 边界与反思：不做什么（RAG/重编排/生产级），为什么这些限制反而是优点。
7. 结尾 CTA：star / 提 issue / 一起来把叙事做得更好。

### 草稿 2：知乎（叙事向）

**标题：三国 × AI：当诸葛亮变成 LLM 军师，五虎将变成 Agent 团队**

> 摘要段（可作开头）：
> 「运筹帷幄之中，决胜千里之外」——这是诸葛亮，也是每一个 AI Agent 编排系统
> 想要做到的事：把大问题拆成小任务，派给最合适的人，最后汇总成一份漂亮的战报。
> 我于是想：为什么不干脆按三国的方式做一个 Agent 框架？主公是用户，军师府是调度器，
> 五虎将各司其职，诸葛亮则是一个真正接了 LLM 的军师——你问它任何问题，它真的会回答。

正文结构建议：
1. 从一句诗讲起：为什么三国叙事和 Agent 编排如此契合。
2. 概念对照表：三国角色 ↔ 工程组件（表格）。
3. 演示场景：与诸葛亮的一问一答 + 多轮对话录屏/截图。
4. 技术深水区：messages 数组透传、Provider 驱动式设计、为什么 v0.2.0 先不做 RAG。
5. 踩坑记录：修复循环依赖、失效依赖版本（jaeger）等骨架「看着完整其实编译不过」的经历。
6. 结尾：开源地址 + 希望一起把它做成「50 star 的小而美」。

### 发布节奏建议

- v0.2.0 tag + Release → 当天发掘金草稿 1
- 3 天内补知乎草稿 2 + 录屏
- 1 周内投 awesome 清单（等 star 稳定后，避免「裸投」）
- 6 个月 50★ gate：数据观察期；<50 则归档为作品集（止损纪律见架构文档）
