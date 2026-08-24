# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased] - M3 (v0.2.0 候选)

### Added
- **多轮交互模式**（`examples/longzhong`）：新增 `--interactive`，stdin 循环对话并
  保留历史；历史为纯内存实现（`pkg/llm.History`，线程安全，零新依赖）
- **多轮历史透传**（`pkg/llm` / `pkg/generals`）：`KongMingHandler` 支持从
  `order.Context["history"]` 读取 `*llm.History`，messages 数组原样透传给
  Provider（不截断、不重排）；战报新增 `turns` 字段便于观察历史长度
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
