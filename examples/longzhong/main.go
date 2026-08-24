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
//   go run ./examples/longzhong/main.go --interactive --save ./session.json   # 结束/退出时保存会话
//   go run ./examples/longzhong/main.go --load ./session.json --interactive   # 加载会话继续对话
//   go run ./examples/longzhong/main.go --tool calc --ask "计算 123*456"  # 内置计算器工具（安全求值）
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
	"unicode"

	"github.com/zhuge/kongming/pkg/cmd_center"
	"github.com/zhuge/kongming/pkg/generals"
	"github.com/zhuge/kongming/pkg/knowledge"
	"github.com/zhuge/kongming/pkg/llm"
	"github.com/zhuge/kongming/pkg/session"
	"github.com/zhuge/kongming/pkg/tools"
	"go.uber.org/zap"
)

func main() {
	mock := flag.Bool("mock", false, "使用本地 Mock Provider 离线演示（无需 API Key）")
	oneShot := flag.String("ask", "", "一问一答模式：直接提问并退出")
	interactive := flag.Bool("interactive", false, "多轮交互模式：stdin 循环对话并保留历史（默认一问一答，无历史）")
	knowledgeDir := flag.String("knowledge", "", "轻量 RAG：本地知识库目录（读取 .md，检索相关段落拼入上下文）")
	jsonOut := flag.Bool("json", false, "结构化 JSON 输出（每轮一个对象，多轮结束时输出 session 汇总）")
	savePath := flag.String("save", "", "多轮/交互会话保存为 JSON 文件（退出时写入）")
	loadPath := flag.String("load", "", "从 JSON 文件加载会话（history + knowledge 配置）继续对话")
	toolName := flag.String("tool", "", "启用内置工具：calc（计算器，识别\"计算 xxx\"表达式并安全求值）；空则不启用")
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

	// 3. 会话加载（可选）：恢复 history 与 knowledge 配置
	var loadedSession *session.Session
	if *loadPath != "" {
		var err error
		loadedSession, err = session.Load(*loadPath)
		if err != nil {
			fmt.Printf("❌ 会话加载失败: %v\n", err)
			os.Exit(1)
		}
		printlnHuman(fmt.Sprintf("📂 已加载会话：%s（历史 %d 条）", *loadPath, len(loadedSession.History)), *jsonOut)
		// 会话中的 knowledge 配置优先于未显式指定的 --knowledge
		if loadedSession.KnowledgeDir != "" && *knowledgeDir == "" {
			*knowledgeDir = loadedSession.KnowledgeDir
		}
	}

	// 4. 轻量 RAG：加载本地知识库（可选）
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
		r := ask(commander, *oneShot, nil, kb, *toolName == "calc")
		emitTurn(r, *jsonOut)
		return
	}

	// 多轮历史（仅内存，不引入依赖）；--load 时从会话恢复
	history := llm.NewHistory()
	if loadedSession != nil && len(loadedSession.History) > 0 {
		history = llm.NewHistoryFromMessages(loadedSession.History)
	}
	mode := "一问一答（无历史）"
	if *interactive {
		mode = "多轮交互（内存历史）"
	}
	printlnHuman(fmt.Sprintf("💬 模式：%s", mode), *jsonOut)

	scanner := bufio.NewScanner(os.Stdin)
	var turns []turnResult // JSON 模式：收集整场会话
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
			r = ask(commander, question, history, kb, *toolName == "calc")
		} else {
			r = ask(commander, question, nil, kb, *toolName == "calc")
		}
		if r != nil {
			emitTurn(r, *jsonOut)
			if *jsonOut {
				turns = append(turns, *r)
			}
		}
	}

	// --save：退出时把当前会话（history + knowledge 配置）写入 JSON
	if *savePath != "" {
		hist := history.Messages()
		if len(hist) > 0 || *knowledgeDir != "" {
			ss := session.New(*knowledgeDir, hist)
			if err := ss.Save(*savePath); err != nil {
				fmt.Printf("❌ 会话保存失败: %v\n", err)
			} else {
				printlnHuman(fmt.Sprintf("💾 会话已保存：%s（历史 %d 条）", *savePath, len(hist)), *jsonOut)
			}
		} else {
			printlnHuman("⚠️  无对话历史，跳过保存（--save 需要至少一轮对话）", *jsonOut)
		}
	}

	// JSON 模式：多轮结束输出 session 汇总
	if *jsonOut && len(turns) > 0 {
		emitSession(turns)
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
	Tool      string   `json:"tool,omitempty"`                // 命中的工具名（如 calc）
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
func ask(commander *cmd_center.Commander, question string, history *llm.History, kb *knowledge.Store, toolCalc bool) *turnResult {
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

	// 工具调用：--tool calc 识别计算表达式并直接求值（不经过 LLM）
	if toolCalc {
		if expr, ok := extractCalcExpr(question); ok {
			val, err := tools.Evaluate(expr)
			if err == nil {
				res.Success = true
				res.Tool = "calc"
				res.Answer = fmt.Sprintf("🧮 计算结果：%s = %s", expr, formatNumber(val))
				if history != nil {
					history.AddUser(question)
					history.AddAssistant(res.Answer)
				}
				return res
			}
			// 表达式存在但求值失败（如除零）：交给 LLM 解释
			res.Tool = "calc"
			order.Context["calc_error"] = err.Error()
		}
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
	if r.Tool != "" {
		fmt.Printf("🛠️  %s（工具：%s）：\n", r.General, r.Tool)
	} else {
		fmt.Printf("🧠 %s：\n", r.General)
	}
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

// extractCalcExpr 从提问中提取计算表达式。
// 支持中文前缀（计算/算一下/帮我算）、英文前缀（calc/calculate），
// 以及裸表达式（如 "123*456"）。返回表达式与是否匹配。
func extractCalcExpr(question string) (string, bool) {
	q := strings.TrimSpace(question)
	if q == "" {
		return "", false
	}
	// 去尾部提问语气词（循环剥到稳定为止，兼容"…等于多少？"这类叠加）
	for changed := true; changed; {
		changed = false
		for _, suf := range []string{"等于多少", "是多少", "等于几", "的结果", "等于", "= ?", "=?", "？", "?"} {
			if strings.HasSuffix(q, suf) {
				q = strings.TrimSpace(strings.TrimSuffix(q, suf))
				changed = true
			}
		}
	}
	// 去前缀
	for _, pre := range []string{"计算", "算一下", "帮我算", "帮我计算", "请问计算", "calculate", "calc", "compute"} {
		if strings.HasPrefix(strings.ToLower(q), pre) {
			q = strings.TrimSpace(q[len(pre):])
			break
		}
	}
	// 去可能残留的冒号/等号
	q = strings.Trim(q, "：:＝=，, ")
	if q == "" {
		return "", false
	}
	// 必须是纯表达式字符：数字、四则、括号、小数点、空白
	for _, r := range q {
		if unicode.IsDigit(r) || strings.ContainsRune("+-*/(). ", r) {
			continue
		}
		return "", false
	}
	// 至少要有一个数字
	hasDigit := false
	for _, r := range q {
		if unicode.IsDigit(r) {
			hasDigit = true
			break
		}
	}
	if !hasDigit {
		return "", false
	}
	return q, true
}

// formatNumber 数字格式化：整数不带小数点
func formatNumber(v float64) string {
	if v == float64(int64(v)) {
		return fmt.Sprintf("%d", int64(v))
	}
	return fmt.Sprintf("%g", v)
}

// llmEnvModel 读取当前模型配置（仅用于提示）
func llmEnvModel() string {
	if m := os.Getenv(llm.EnvModel); m != "" {
		return m
	}
	return "gpt-4o-mini"
}
