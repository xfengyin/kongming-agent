# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased] - M7 (v0.6.0 候选)

### Added
- **工具调用：内置计算器**（`pkg/tools`）：`Evaluate` 安全表达式求值——递归下降
  解析，仅支持数字/四则（+ - * /）/括号/小数点/一元正负号，绝不 eval 任意代码；
  除零、非法字符、空表达式返回明确错误；`Calculator` 实现工具接口（Name=calc）
- **demo 接入**（`examples/longzhong`）：新增 `--tool calc`，识别「计算/算一下/
  帮我算/calc/calculate + 表达式」或裸表达式（含"…等于多少？"等语气词），直接
  求值并作为工具结果返回；非计算问题或求值失败（如除零）自动回落 LLM 流程；
  战报新增 `tool` 字段（JSON 输出可见）

### Tests
- `pkg/tools`：基础四则/优先级/括号/一元符号/小数/嵌套括号/空白容忍、错误
  （空/非法/除零/括号不匹配/连续运算符）、工具接口、危险表达式拒绝
  （os.Exit/`ls`/exec 等全部拒绝）
- `examples/longzhong`：`extractCalcExpr` 提取用例（前缀/裸表达式/语气词/
  非计算不匹配/含非法字符不匹配）

## [Unreleased] - M6 (v0.5.0 候选)

### Added
- **会话保存/加载**（`examples/longzhong`）：新增 `--save <file>` / `--load <file>`，
  多轮/交互会话以 JSON 持久化（history + knowledge 目录配置），加载后继续对话；
  会话文件为临时文件 + rename 原子写入，损坏/版本不兼容文件给出明确报错
- **会话模块**（`pkg/session`）：`New/Save/Load`，版本化 JSON 格式（当前 v1），
  `llm.History` 新增 `NewHistoryFromMessages` 支持会话历史还原

### Tests
- `pkg/session`：保存/加载往返、加载不存在文件、损坏 JSON、版本不兼容、
  nil 会话、时间戳更新、历史还原后继续对话（7 用例）
- `pkg/llm`：`NewHistoryFromMessages` 还原断言

## [Unreleased] - M5 (v0.4.0 候选)

### Added
- **结构化 JSON 输出**（`examples/longzhong`）：新增 `--json`，每轮输出一个
  JSON 对象（`type/question/general/answer/model/success/turns/retrieved_knowledge`），
  多轮结束输出 `session` 汇总（total_turns/questions/turns），可直接 pipe 给
  jq / 脚本集成
- **发布启动包**（`docs/LAUNCH_PACK.md`）：D-Day 执行清单、掘金/知乎/HN/
  Awesome 四渠道最终稿、发布后 tracking 表、60–90s 录屏脚本（含 RAG/JSON 场景）

## [Unreleased] - M3 (v0.2.0 候选)

### Added
- **多轮交互模式**（`examples/longzhong`）：新增 `--interactive`，stdin 循环对话并
  保留历史；历史为纯内存实现（`pkg/llm.History`，线程安全，零新依赖）
- **多轮历史透传**（`pkg/llm` / `pkg/generals`）：`KongMingHandler` 支持从
  `order.Context["history"]` 读取 `*llm.History`，messages 数组原样透传给
  Provider（不截断、不重排）；战报新增 `turns` 字段便于观察历史长度
- **轻量 RAG 知识库**（`pkg/knowledge`）：`Load(dir)` 读取本地 .md 按段落切分，
  `Search(query, limit)` 词频/包含匹配检索（零向量库、零外部依赖）；示例知识库
  `knowledge/sanguo.md`（隆中对/空城计/草船借箭/木牛流马/七擒七纵 5 条）
- **demo 接入 RAG**：`examples/longzhong --knowledge <dir>` 查询前检索相关段落
  拼入系统上下文（可叠加 `--interactive` 多轮）
- **推广文档**（`docs/PROMOTION.md`）：三国×AI 卖点、目标人群、Awesome/社区
  投稿清单、掘金/知乎两篇推广文草稿
- **演示脚本**（`docs/demo.md`）：离线 mock / 真实 LLM / 多轮交互三条演示路径，
  含预期输出示例与录屏建议

### Tests
- `pkg/llm/history_test.go`：History 追加/副本隔离/Reset/并发安全
- `pkg/generals/llm_test.go`：多轮历史透传断言（recordingProvider 校验
  system+历史+当前问题 的完整消息序列）

## [v0.1.0] - M1 (v0.1.0 候选)

### Added
- **LLM Provider 适配层**（`pkg/llm`）：`LLMProvider` 接口 + OpenAI 兼容适配器
  （DeepSeek / 通义 / OpenAI 通用），环境变量 `KONGMING_API_KEY` /
  `KONGMING_BASE_URL` / `KONGMING_MODEL` / `KONGMING_PROVIDER`
- **军师诸葛亮（LLM 将领）**（`pkg/generals/llm.go`）：五虎将之外的 LLM 驱动
  军师，内置人设锦囊，执行真实 LLM 调用
- **隆中对示例**（`examples/longzhong`）：`go run` 即可与诸葛亮真实对话，
  支持 `--mock` 离线演示与 `--ask` 一问一答
- **军师府点将支持**：`strategy.Generals` 显式点名将领；未点名时按目标关键词
  自动匹配五虎将技能
- **核心测试**：cmd_center（点将/自动选将/八卦阵选型）、generals（LLM 将领、
  缺 Key 引导）、llm（httptest HTTP 契约）、courier、observatory

### Changed
- **修复编译阻塞**：`cmd_center` ⇄ `generals` 循环依赖（共享类型抽取至
  `pkg/core`）、`Commander` 接口/结构体重名、`Dispatch` 丢失点名将领、
  tactical Action 无匹配技能、测试文件引用未导入类型
- **移除失效依赖**：`go.opentelemetry.io/otel/exporters/jaeger`（声明的版本
  不存在且上游已弃用），观测台回归纯 Prometheus 指标并新增 LLM 调用指标
- **README 能力对齐**：如实标注「已实装 / Roadmap」，新增双语 README
  （README.md 英文 + README.zh-CN.md 中文）
- 五虎将测试改为 5 将 + LLM 池 6 将（五虎将 + 诸葛亮）

## [1.0.0] - 2024-01-01（历史骨架版本）

### Added
- Initial release
- Core architecture with Commander, Generals, Bagua Engine, Vault, Courier
- Five Tiger Generals system (GuanYu, ZhangFei, ZhaoYun, MaChao, HuangZhong)
- Eight Trigrams Formation Engine (Tiangai, Dizai, Fengyang, Yunzhui, Longfei)
- Strategy Vault for skill management
- Courier for message delivery
- Repeater with retry and circuit breaker
- Prometheus metrics integration
- Graceful shutdown
- Docker and docker-compose support
- CI/CD with GitHub Actions

> 注：历史版本宣称的 OpenTelemetry tracing 依赖版本无法解析（jaeger exporter
> 模块版本号错误），且整体骨架存在循环依赖无法编译；M1 已修复。
