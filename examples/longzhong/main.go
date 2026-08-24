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
//
// 无 Key 时可用 --mock 离线演示：
//   go run ./examples/longzhong/main.go --mock --interactive --knowledge ./knowledge

package main

import (
	"bufio"
	"context"
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
		fmt.Println("⚙️  离线演示模式（Mock Provider）")
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
		fmt.Printf("⚙️  Provider: %s | Model: %s\n", provider.Name(), llmEnvModel())
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
		fmt.Printf("📚 知识库已加载：%s（%d 个段落）\n", kb.Dir(), kb.Count())
	}

	fmt.Println()
	fmt.Println("=== 隆中对 · 孔明军师 ===")
	fmt.Println("主公有何要事相询？（输入 exit 退出）")
	fmt.Println()

	if *oneShot != "" {
		ask(commander, *oneShot, nil, kb)
		return
	}

	// 多轮历史（仅内存，不引入依赖）
	history := llm.NewHistory()
	mode := "一问一答（无历史）"
	if *interactive {
		mode = "多轮交互（内存历史）"
	}
	fmt.Printf("💬 模式：%s\n", mode)
	fmt.Println()

	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("主公> ")
		if !scanner.Scan() {
			break
		}
		question := strings.TrimSpace(scanner.Text())
		if question == "" {
			continue
		}
		if question == "exit" || question == "quit" || question == "退出" {
			fmt.Println("亮告退。后会有期！")
			break
		}

		// 多轮：历史随军令透传给诸葛亮；非多轮则不传
		if *interactive {
			ask(commander, question, history, kb)
		} else {
			ask(commander, question, nil, kb)
		}
		fmt.Println()
	}
}

// ask 向军师诸葛亮问计并打印战报。
// history 非 nil 时启用多轮：把历史挂到军令 Context 透传给诸葛亮，
// 并在战后把「主公问题 + 军师回复」追加进历史，供下一轮使用。
// kb 非 nil 时启用轻量 RAG：先检索相关段落拼入军令 Context 作为参考。
func ask(commander *cmd_center.Commander, question string, history *llm.History, kb *knowledge.Store) {
	ctx := context.Background()
	order := cmd_center.NewMilitaryOrder("隆中对", question, cmd_center.PriorityNormal)
	order.Strategy.Generals = []string{"kongming"} // 点将：诸葛亮
	if history != nil {
		order.Context["history"] = history
	}
	if kb != nil {
		if paras := kb.Search(question, 3); len(paras) > 0 {
			order.Context["knowledge"] = formatKnowledge(paras)
			fmt.Printf("📚 检索到 %d 段相关知识（%s）\n", len(paras), paras[0].Title)
		}
	}

	report, err := commander.Dispatch(ctx, order)
	if err != nil {
		fmt.Printf("❌ 军师府调度失败: %v\n", err)
		return
	}

	for _, gr := range report.Generals {
		if !gr.Success {
			fmt.Printf("❌ %s：%s\n", gr.GeneralName, gr.Message)
			continue
		}
		fmt.Printf("🧠 %s：\n", gr.GeneralName)
		var answer string
		if a, ok := gr.Data["answer"].(string); ok {
			answer = a
			fmt.Println(answer)
		} else {
			answer = gr.Message
			fmt.Println(answer)
		}
		// 多轮：把本轮问答追加进历史
		if history != nil {
			history.AddUser(question)
			history.AddAssistant(answer)
		}
		if model, ok := gr.Data["model"].(string); ok && model != "" {
			fmt.Printf("（模型：%s）\n", model)
		}
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
