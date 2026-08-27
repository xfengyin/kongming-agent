# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased] - M9 (v0.8.0 候选) — 彻底重构

### Changed
- **架构瘦身**：16 个包收编为 7 个（agent/config/knowledge/llm/session/tools + cmd），
  约 5900 行减至 ~2500 行；删除全部未接线的"展示性架构"——八卦阵工作流引擎
  （bagua，8 模式只实现 4 个、0 测试）、传令兵消息总线（courier）、异步调度
  （dispatch）、重试/熔断（repeater，构造器拼写错误 NewReperier）、观测台
  （observatory，Prometheus 指标，多数埋点无人调）、锦囊库（strategy_vault，
  LoadFromDir 为 TODO 桩）、三层记忆（internal/memory，goroutine 泄漏）、
  军令/战报域类型（core + cmd_center 整包重复导出）、五虎将罐头池（generals）
- **CLI 升级**：`cmd/kongming` 从空壳（metrics HTTP 服务）升级为真正的产品入口，
  整合原 `examples/longzhong` demo 的全部能力（--mock/--ask/--interactive/
  --knowledge/--json/--save/--load/--tool/--config），新增 `--history-limit`
  （按"轮"截断）与 `--version`；`run(args, stdin, stdout, stderr) int` 可测试化，
  JSON 契约（turnResult/sessionResult）原样保留
- **新核心包**（`pkg/agent`）：`Agent.Ask(ctx, question) -> Reply` 统一编排
  工具预检 → RAG 检索 → 消息组装 → LLM 调用 → 历史记录；修复原 `Commander.Dispatch`
  "全败仍报胜"、`ListOrders` 无法过滤 Pending、`WuHuPool.Execute` 无锁改共享状态
  （数据竞争）等 bug；错误改为显式返回（不再吞成假战报）
- **模块路径对齐**：`github.com/zhuge/kongming` → `github.com/xfengyin/kongming-agent`
  （对齐仓库真实地址，修复 `go install` 失效）；所有 import 同步更新
- **依赖收编**：移除 prometheus/client_golang、google/uuid、go.uber.org/zap
  （日志整体砍掉，Agent 不持 logger）；直接依赖仅剩 `gopkg.in/yaml.v3`
- **配置收敛**：删除 `config.Apply()` 写环境变量通道（改为直接
  `NewOpenAIProvider(cfg.*)`）；新增 `history_limit` 配置字段；删除剧场配置
  `configs/kongming.yaml`（server/features/generals/vault/bagua/courier/repeater 全为死配置）
- **修复**：`pkg/knowledge` CRLF 换行导致 `.md` 整文件塌成 1 段（归一化换行符）；
  `KONGMING_PROVIDER` 语义从"指标标签"改为"显示名"
- **部署收敛**：删除 docker-compose / deployments（Prometheus/Grafana）/ healthcheck
  （curl :9090）；Dockerfile 改为一次性离线 demo 镜像

### Removed
- 包：bagua、courier、dispatch、repeater、observatory、strategy_vault、core、
  cmd_center、generals、internal/memory、pkg/llm.History、examples/longzhong
- 依赖：prometheus/client_golang、google/uuid、go.uber.org/zap
- 文件：configs/、deployments/、docker-compose.yml、scripts/healthcheck.sh、
  docs/LAUNCH_PACK.md、docs/PROMOTION.md（发布/营销文档，README 已不引用）

### Added
- 补 MIT LICENSE（版权人 COMTool Team）

### Added
- `pkg/tools` 工具注册表（Tool 接口 + Registry，first-match-wins）；
  `extractCalcExpr` 从 demo 移入计算器工具（含 13 用例迁移）
- `examples/quickstart` 重写为库嵌入演示（单轮/多轮/工具/RAG）
- `pkg/agent` 测试（97%+）：多轮历史、知识注入、工具短路与回落、历史截断、
  并发安全（-race）、ctx 取消
- `cmd/kongming/run_test.go`：CLI 集成测试 14 例（JSON 契约/会话往返/工具/配置/历史截断）

## [Unreleased] - M8 (v0.7.0 候选)

### Added
- **配置文件支持**（`pkg/config`）：`Load(path)` 读取 YAML/JSON 配置（按扩展名
  自动识别），支持 LLM 连接（api_key/base_url/model/provider）、知识库目录
  （knowledge_dir）、默认工具（tool）；优先级为环境变量 > 配置文件；
  `Apply()` 仅写入环境变量为空的项（不覆盖已有 env）
- **demo 接入**（`examples/longzhong`）：新增 `--config <file>`，加载后把配置
  写入环境变量并合并知识库目录/默认工具（flag 显式指定时优先于配置）；未指定
  `--config` 时行为不变
- **示例配置**（`config.example.yaml`）：含全部字段与注释、JSON 等价示例
- **依赖**：引入 `gopkg.in/yaml.v3`（YAML 解析）

### Tests
- `pkg/config`：YAML 加载、JSON 加载、环境变量优先、缺失文件、非法格式
  （YAML/JSON 各一）、空路径、Apply 写入、Apply 不覆盖已有 env（9 用例）

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
