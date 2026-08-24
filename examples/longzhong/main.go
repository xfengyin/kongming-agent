// 隆中对 - 孔明军师真实 LLM 对话示例
// 运筹帷幄之中，决胜千里之外
//
// 用法：
//   export KONGMING_API_KEY=sk-xxx          # 必填（DeepSeek/OpenAI/通义任一 OpenAI 兼容 Key）
//   export KONGMING_BASE_URL=https://api.deepseek.com/v1   # 可选，默认 OpenAI
//   export KONGMING_MODEL=deepseek-chat      # 可选，默认 gpt-4o-mini
//   go run ./examples/longzhong/main.go              # 交互模式（一问一答，无历史）
//   go run ./examples/longzhong/main.go --interactive  # 多轮交互（内存历史）
//   go run ./examples/longzhong/main.go --ask "问题"   # 一问一答
//   go run ./examples/longzhong/main.go --knowledge ./knowledge  # 轻量 RAG：检索知识库拼入上下文
//   go run ./examples/longzhong/main.go --json         # 结构化 JSON 输出（便于集成/录屏）
//
// 无 Key 时可用 --mock 离线演示：
//   go run ./examples/longzhong/main.go --mock --interactive --knowledge ./knowledge --json

package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/zhuge/kongming/pkg/cmd_center"
	"github.com/zhuge/kongming/pkg/generals"
	"github.com/zhuge/kongming/pkg/knowledge"
	"github.com/zhuge/kongming/pkg/llm"
	"go.uber.org/zap"
)

func main() {
	mock := flag.Bool("mock", false, "使用本地 Mock Provider 离线演示（无需 API Key）")
	oneShot := flag.String("ask", "", "一问一答模式：直接提问并退出")
	interactive := flag.Bool("interactive", false, "多轮交互模式：stdin 循环对话并保留历史（默认一问一答，无历史）")
	knowledgeDir := flag.String("knowledge", "", "轻量 RAG：本地知识库目录（读取 .md，检索相关段落拼入上下文）")
	jsonOut := flag.Bool("json", false, "结构化 JSON 输出（每轮一个对象，多轮结束时输出 session 汇总）")
	flag.Parse()

	logger, err := zap.NewDevelopment()
	if err != nil {
		log.Fatalf("初始化日志失败: %v", err)
	}
	defer logger.Sync()

	// 1. 选择 Provider
	var provider llm.Provider
	if *mock {
		provider = &llm.MockProvider{}
		printlnHuman("⚙️  离线演示模式（Mock Provider）", *jsonOut)
	} else {
		p, err := llm.NewOpenAIProviderFromEnv()
		if err != nil {
			fmt.Println("❌ " + err.Error())
			fmt.Println()
			fmt.Println("请先配置环境变量（任选一家 OpenAI 兼容服务）：")
			fmt.Println("  export KONGMING_API_KEY=sk-xxx")
			fmt.Println("  export KONGMING_BASE_URL=https://api.deepseek.com/v1   # 可选")
			fmt.Println("  export KONGMING_MODEL=deepseek-chat                    # 可选")
			fmt.Println()
			fmt.Println("或使用离线演示：go run ./examples/longzhong/main.go --mock")
			os.Exit(1)
		}
		provider = p
		printlnHuman(fmt.Sprintf("⚙️  Provider: %s | Model: %s", provider.Name(), llmEnvModel()), *jsonOut)
	}

	// 2. 组建军师府：五虎将 + 军师诸葛亮（LLM 驱动）
	pool := generals.NewWuHuPoolWithLLM(provider)
	commander := cmd_center.NewCommanderWithPool(logger, pool)

	// 3. 轻量 RAG：加载本地知识库（可选）
	var kb *knowledge.Store
	if *knowledgeDir != "" {
		var err error
		kb, err = knowledge.Load(*knowledgeDir)
		if err != nil {
			fmt.Printf("❌ 知识库加载失败: %v\n", err)
			os.Exit(1)
		}
		printlnHuman(fmt.Sprintf("📚 知识库已加载：%s（%d 个段落）", kb.Dir(), kb.Count()), *jsonOut)
	}

	printlnHuman("", *jsonOut)
	printlnHuman("=== 隆中对 · 孔明军师 ===", *jsonOut)
	printlnHuman("主公有何要事相询？（输入 exit 退出）", *jsonOut)

	if *oneShot != "" {
		r := ask(commander, *oneShot, nil, kb)
		emitTurn(r, *jsonOut)
		return
	}

	// 多轮历史（仅内存，不引入依赖）
	history := llm.NewHistory()
	mode := "一问一答（无历史）"
	if *interactive {
		mode = "多轮交互（内存历史）"
	}
	printlnHuman(fmt.Sprintf("💬 模式：%s", mode), *jsonOut)

	scanner := bufio.NewScanner(os.Stdin)
	var session []turnResult // JSON 模式：收集整场会话
	for {
		if !*jsonOut {
			fmt.Print("主公> ")
		}
		if !scanner.Scan() {
			break
		}
		question := strings.TrimSpace(scanner.Text())
		if question == "" {
			continue
		}
		if question == "exit" || question == "quit" || question == "退出" {
			printlnHuman("亮告退。后会有期！", *jsonOut)
			break
		}

		// 多轮：历史随军令透传给诸葛亮；非多轮则不传
		var r *turnResult
		if *interactive {
			r = ask(commander, question, history, kb)
		} else {
			r = ask(commander, question, nil, kb)
		}
		if r != nil {
			emitTurn(r, *jsonOut)
			if *jsonOut {
				session = append(session, *r)
			}
		}
	}

	// JSON 模式：多轮结束输出 session 汇总
	if *jsonOut && len(session) > 0 {
		emitSession(session)
	}
}

// turnResult 单轮对话的结构化结果（JSON 输出用）
type turnResult struct {
	Type      string   `json:"type"`                          // 固定 "turn"
	Question  string   `json:"question"`                      // 主公提问
	General   string   `json:"general"`                       // 应战将领（诸葛亮）
	Answer    string   `json:"answer"`                        // 军师回复
	Model     string   `json:"model,omitempty"`               // 模型名
	Success   bool     `json:"success"`                       // 是否成功
	Turns     int      `json:"turns"`                         // 本轮发给 LLM 的总消息条数（含人设/知识/历史）
	Knowledge []string `json:"retrieved_knowledge,omitempty"` // RAG 检索到的段落标题
	Error     string   `json:"error,omitempty"`               // 失败原因
}

// sessionResult 整场会话汇总（JSON 输出，多轮结束时打印）
type sessionResult struct {
	Type       string       `json:"type"` // 固定 "session"
	TotalTurns int          `json:"total_turns"`
	Questions  []string     `json:"questions"`
	Turns      []turnResult `json:"turns"`
}

// ask 向军师诸葛亮问计，返回结构化结果。
// history 非 nil 时启用多轮：把历史挂到军令 Context 透传给诸葛亮，
// 并在战后把「主公问题 + 军师回复」追加进历史，供下一轮使用。
// kb 非 nil 时启用轻量 RAG：先检索相关段落拼入军令 Context 作为参考。
func ask(commander *cmd_center.Commander, question string, history *llm.History, kb *knowledge.Store) *turnResult {
	ctx := context.Background()
	order := cmd_center.NewMilitaryOrder("隆中对", question, cmd_center.PriorityNormal)
	order.Strategy.Generals = []string{"kongming"} // 点将：诸葛亮
	if history != nil {
		order.Context["history"] = history
	}

	res := &turnResult{
		Type:     "turn",
		Question: question,
		General:  "诸葛亮",
	}

	// 轻量 RAG：检索相关段落
	if kb != nil {
		paras := kb.Search(question, 3)
		if len(paras) > 0 {
			order.Context["knowledge"] = formatKnowledge(paras)
			for _, p := range paras {
				if p.Title != "" {
					res.Knowledge = append(res.Knowledge, p.Title)
				}
			}
		}
	}

	report, err := commander.Dispatch(ctx, order)
	if err != nil {
		res.Success = false
		res.Error = err.Error()
		return res
	}

	for _, gr := range report.Generals {
		if !gr.Success {
			res.Success = false
			res.Error = gr.Message
			continue
		}
		res.Success = true
		if a, ok := gr.Data["answer"].(string); ok {
			res.Answer = a
		} else {
			res.Answer = gr.Message
		}
		if model, ok := gr.Data["model"].(string); ok {
			res.Model = model
		}
		if turns, ok := gr.Data["turns"].(int); ok {
			res.Turns = turns
		}
		// 多轮：把本轮问答追加进历史
		if history != nil {
			history.AddUser(question)
			history.AddAssistant(res.Answer)
		}
	}
	return res
}

// emitTurn 输出单轮结果：JSON 模式打印对象，人类模式打印战报
func emitTurn(r *turnResult, jsonOut bool) {
	if r == nil {
		return
	}
	if jsonOut {
		b, err := json.Marshal(r)
		if err != nil {
			fmt.Printf(`{"type":"error","message":%q}`+"\n", err.Error())
			return
		}
		fmt.Println(string(b))
		return
	}

	if !r.Success {
		fmt.Printf("❌ %s：%s\n", r.General, r.Error)
		return
	}
	fmt.Printf("🧠 %s：\n", r.General)
	fmt.Println(r.Answer)
	if len(r.Knowledge) > 0 {
		fmt.Printf("📚 参考知识：%s\n", strings.Join(r.Knowledge, "、"))
	}
	if r.Model != "" {
		fmt.Printf("（模型：%s | 消息 %d 条）\n", r.Model, r.Turns)
	}
}

// emitSession 输出整场会话汇总（JSON 模式，多轮结束时）
func emitSession(session []turnResult) {
	summary := sessionResult{
		Type:       "session",
		TotalTurns: len(session),
		Questions:  make([]string, 0, len(session)),
		Turns:      session,
	}
	for _, r := range session {
		summary.Questions = append(summary.Questions, r.Question)
	}
	b, err := json.Marshal(summary)
	if err != nil {
		fmt.Printf(`{"type":"error","message":%q}`+"\n", err.Error())
		return
	}
	fmt.Println(string(b))
}

// printlnHuman 人类友好输出辅助：JSON 模式跳过装饰行
func printlnHuman(s string, jsonOut bool) {
	if !jsonOut && s != "" {
		fmt.Println(s)
	}
}

// formatKnowledge 把检索到的段落拼成参考上下文（标题 + 正文）
func formatKnowledge(paras []knowledge.Paragraph) string {
	var sb strings.Builder
	sb.WriteString("以下是主公的军师知识库中与本问相关的记载，可参考但不必逐字引用：\n")
	for i, p := range paras {
		if i > 0 {
			sb.WriteString("\n")
		}
		if p.Title != "" {
			sb.WriteString("【" + p.Title + "】\n")
		}
		sb.WriteString(p.Content)
	}
	return sb.String()
}

// llmEnvModel 读取当前模型配置（仅用于提示）
func llmEnvModel() string {
	if m := os.Getenv(llm.EnvModel); m != "" {
		return m
	}
	return "gpt-4o-mini"
}
