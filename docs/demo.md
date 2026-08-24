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
