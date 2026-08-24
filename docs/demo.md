# 演示脚本（demo）— 孔明军师 Kongming

> 适用版本：v0.2.0。三种演示路径：离线 mock / 真实 LLM / 多轮交互。
> 建议录屏前先在本机跑通一遍，核对预期输出。

---

## 0. 准备

```bash
cd kongming-agent
# Go 1.21+
go version
```

## 1. 路径 A：离线演示（无需 Key，推荐录屏起步）

```bash
go run ./examples/longzhong/main.go --mock --ask "天下大势，分久必合，亮以为如何？"
```

**预期输出（节选）：**

```
⚙️  离线演示模式（Mock Provider）

=== 隆中对 · 孔明军师 ===
主公有何要事相询？（输入 exit 退出）

🧠 诸葛亮：
[mock] 主公问：天下大势，分久必合，亮以为如何？
亮答：此乃天机，容亮细想——离线演示模式，请配置 KONGMING_API_KEY 接入真实 LLM。（本轮收到 2 条消息）
（模型：mock-model）
```

## 2. 路径 B：真实 LLM（任意 OpenAI 兼容 Key）

```bash
export KONGMING_API_KEY=sk-xxx
export KONGMING_BASE_URL=https://api.deepseek.com/v1   # DeepSeek 示例
export KONGMING_MODEL=deepseek-chat

go run ./examples/longzhong/main.go --ask "请分析当前 AI Agent 赛道的格局"
```

**预期输出（节选）：**

```
⚙️  Provider: openai-compatible | Model: deepseek-chat

=== 隆中对 · 孔明军师 ===

🧠 诸葛亮：
（真实 LLM 输出，格式大致为：局势判断 / 关键矛盾 / 行动建议 三段）
（模型：deepseek-chat）
```

> 无 Key 时应看到清晰的引导错误：
> `❌ 未配置 LLM API Key（请设置 KONGMING_API_KEY 环境变量）` + 配置示例 + `--mock` 提示。

## 3. 路径 C：多轮交互（v0.2.0 新能力）

```bash
# 离线多轮
go run ./examples/longzhong/main.go --mock --interactive

# 真实多轮（配好 Key 后）
go run ./examples/longzhong/main.go --interactive
```

**交互脚本（含预期输出）：**

```
$ go run ./examples/longzhong/main.go --mock --interactive
⚙️  离线演示模式（Mock Provider）

=== 隆中对 · 孔明军师 ===
主公有何要事相询？（输入 exit 退出）

💬 模式：多轮交互（内存历史）

主公> 第一问：天下大势如何？
🧠 诸葛亮：
[mock] 主公问：第一问：天下大势如何？
亮答：此乃天机……（本轮收到 2 条消息）   ← 第 1 轮：system + user = 2 条
（模型：mock-model）

主公> 第二问：那刘备当如何自处？
🧠 诸葛亮：
[mock] 主公问：第二问：那刘备当如何自处？
亮答：此乃天机……（本轮收到 4 条消息）   ← 第 2 轮：system + 上一轮问答 + 新问题 = 4 条，历史已透传
（模型：mock-model）

主公> 第三问：若曹操南下呢？
🧠 诸葛亮：
[mock] 主公问：第三问：若曹操南下呢？
亮答：此乃天机……（本轮收到 6 条消息）   ← 第 3 轮：历史持续累积
（模型：mock-model）

主公> exit
亮告退。后会有期！
```

> 🔍 **演示要点**：注意「本轮收到 N 条消息」随轮次递增（2 → 4 → 6），
> 直观证明多轮历史通过 messages 数组原样透传给了 Provider。

## 3.5 路径 D：知识库 RAG（v0.2.0+）

```bash
# 内置三国知识库（knowledge/sanguo.md，5 个典故段落）
go run ./examples/longzhong/main.go --knowledge ./knowledge \
  --ask "司马懿兵临城下，如何用空城计退敌？"

# 多轮 + RAG 叠加
go run ./examples/longzhong/main.go --mock --interactive --knowledge ./knowledge
```

**预期输出（节选）：**

```
⚙️  离线演示模式（Mock Provider）
📚 知识库已加载：./knowledge（5 个段落）

=== 隆中对 · 孔明军师 ===

📚 检索到 1 段相关知识（空城计）
🧠 诸葛亮：
[mock] 主公问：司马懿兵临城下，如何用空城计退敌？
亮答：此乃天机……（本轮收到 3 条消息）  ← 人设 + 知识上下文 + 问题 = 3 条，RAG 注入生效
```

> 🔍 **演示要点**：① 启动时显示「知识库已加载（N 个段落）」；② 提问命中时显示
> 「检索到 N 段相关知识（段落标题）」；③ mock 汇报消息数为 3（人设 + 知识 +
> 问题），对比不带 --knowledge 时的 2 条——直观证明知识已注入 LLM 上下文。
> 换一个知识库未收录的话题（如「量子计算」）则不会检索到段落，消息数回到 2。

## 3.6 路径 E：会话保存 / 加载（v0.5.0+）

```bash
# 场景 A：多轮对话，退出时保存会话
go run ./examples/longzhong/main.go --mock --interactive --save ./session.json
# 输入两问后 exit，预期输出末尾：
# 💾 会话已保存：./session.json（历史 4 条）

# 场景 B：改天加载会话继续对话（历史恢复，消息数接着涨）
go run ./examples/longzhong/main.go --mock --interactive --load ./session.json
# 📂 已加载会话：./session.json（历史 4 条）
# 继续提问后 mock 显示消息数从 4 → 6 → 8 递增，证明历史已恢复

# 场景 C：knowledge 配置随会话保存/恢复
go run ./examples/longzhong/main.go --mock --interactive --knowledge ./knowledge --save ./kb.json
go run ./examples/longzhong/main.go --mock --load ./kb.json --ask "空城计怎么用？"
# 📚 知识库已加载：./knowledge（5 个段落） ← 来自会话配置，无需再传 --knowledge
```

> 🔍 **演示要点**：保存时看「历史 N 条」；加载后继续提问看消息数从 N 继续递增
> （而非重置为 2）——直观证明会话持久化。损坏的会话文件会得到明确报错并退出。

## 3.7 路径 F：工具调用（v0.6.0+，内置计算器）

```bash
# 中文前缀 / 裸表达式 / 语气词都能识别
go run ./examples/longzhong/main.go --mock --tool calc --ask "计算 123*456"
go run ./examples/longzhong/main.go --mock --tool calc --ask "(2+3)*4 等于多少？"
# 非计算问题自动回落 LLM
go run ./examples/longzhong/main.go --mock --tool calc --ask "天下大势如何？"
```

**预期输出（命中时）：**

```
🛠️  诸葛亮（工具：calc）：
🧮 计算结果：123*456 = 56088
```

> 🔍 **演示要点**：① 计算类提问显示「工具：calc」与算式结果，且不经过 LLM
> （速度极快）；② 非计算提问走原 LLM 流程（显示「🧠 诸葛亮」）；③ 安全边界——
> 仅数字/四则/括号求值，`计算 sqrt(4)`、`计算 123 的平方` 等不会命中计算器，
> 也不会执行任何代码。

## 3.8 路径 G：配置文件（v0.7.0+）

```bash
# 1. 复制示例并编辑（LLM 连接 / 知识库目录 / 默认工具）
cp config.example.yaml my-config.yaml
# 2. 用配置文件启动（环境变量仍优先）
go run ./examples/longzhong/main.go --mock --config my-config.yaml
# 3. 也支持 JSON：--config config.json
```

**预期输出（配置了 knowledge + tool calc 时）：**

```
⚙️  已加载配置：my-config.yaml
📚 知识库已加载：./knowledge（5 个段落）
主公> 计算 123*456
🛠️  诸葛亮（工具：calc）：
🧮 计算结果：123*456 = 56088
```

> 🔍 **演示要点**：① 启动时显示「已加载配置」；② 配置中的知识库目录与默认工具
> 自动生效（无需再传 --knowledge / --tool）；③ 优先级为 flag > 环境变量 >
> 配置文件——可现场演示「export KONGMING_MODEL=xxx 覆盖配置」；④ 配置文件缺失
> 或格式非法会得到明确报错并退出。

## 4. 服务模式（可选补充）

```bash
make build && ./bin/kongming
# 指标:   http://localhost:9090/metrics
# 健康:   curl http://localhost:9090/health  → OK
```

## 5. 录屏建议

1. 开场 5s：展示 README 架构图（军师府 + 五虎将 + 诸葛亮）。
2. 路径 A 快速过一遍离线对话。
3. 路径 C 展示多轮「消息数递增」的关键帧（暂停 1s 让数字被看清）。
4. 结尾 3s：展示 GitHub 地址 + star 引导。
5. 建议 60–90s，竖屏 9:16 或横屏 16:9 均可；配轻音乐或三国题材 BGM。
