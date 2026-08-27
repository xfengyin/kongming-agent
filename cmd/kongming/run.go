// Kongming 孔明军师系统 - CLI 装配与运行
//
// 职责：
//   - flag 解析、配置/会话/知识库装配、Provider 选择；
//   - REPL 交互循环与单轮 --ask 模式；
//   - 人类可读与 JSON 两种输出（JSON 契约见 turnResult/sessionResult，勿破坏）。
//
// 设计约束：
//   - 领域编排全部委托 pkg/agent，本文件只做展示与装配；
//   - 非交互模式每轮 Reset()，避免连续 --ask 串上下文；
//   - JSON 模式下 stdout 只输出契约对象，装饰/错误走 stderr；
//   - run() 通过返回值报告退出码，绝不调用 os.Exit（保证可测试）。

package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/zhuge/kongming/pkg/agent"
	"github.com/zhuge/kongming/pkg/config"
	"github.com/zhuge/kongming/pkg/knowledge"
	"github.com/zhuge/kongming/pkg/llm"
	"github.com/zhuge/kongming/pkg/session"
	"github.com/zhuge/kongming/pkg/tools"
)

// version 当前版本号（与 CHANGELOG 同步）
const version = "0.8.0"

// runFlags CLI 参数集合
type runFlags struct {
	mock         bool
	oneShot      string
	interactive  bool
	knowledgeDir string
	jsonOut      bool
	savePath     string
	loadPath     string
	toolName     string
	configPath   string
	historyLimit int
}

// run 执行 CLI 主流程，返回进程退出码。
// 拆出独立函数以便 run_test.go 直接做集成测试（不起子进程）。
func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("kongming", flag.ContinueOnError)
	fs.SetOutput(stderr)

	flags := &runFlags{}
	fs.BoolVar(&flags.mock, "mock", false, "使用本地 Mock Provider 离线演示（无需 API Key）")
	fs.StringVar(&flags.oneShot, "ask", "", "一问一答模式：直接提问并退出")
	fs.BoolVar(&flags.interactive, "interactive", false, "多轮交互模式：stdin 循环对话并保留历史（默认一问一答，无历史）")
	fs.StringVar(&flags.knowledgeDir, "knowledge", "", "轻量 RAG：本地知识库目录（读取 .md，检索相关段落拼入上下文）")
	fs.BoolVar(&flags.jsonOut, "json", false, "结构化 JSON 输出（每轮一个对象，多轮结束时输出 session 汇总）")
	fs.StringVar(&flags.savePath, "save", "", "多轮/交互会话保存为 JSON 文件（退出时写入）")
	fs.StringVar(&flags.loadPath, "load", "", "从 JSON 文件加载会话（history + knowledge 配置）继续对话")
	fs.StringVar(&flags.toolName, "tool", "", "启用内置工具：calc（计算器，识别\"计算 xxx\"表达式并安全求值）；空则不启用")
	fs.StringVar(&flags.configPath, "config", "", "YAML/JSON 配置文件路径（环境变量优先；知识库目录/工具可由此设置）")
	fs.IntVar(&flags.historyLimit, "history-limit", 0, "多轮历史按\"轮\"截断上限（0=不限，默认保留全部）")
	showVersion := fs.Bool("version", false, "打印版本号并退出")

	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 2
	}
	if *showVersion {
		fmt.Fprintf(stdout, "kongming %s\n", version)
		return 0
	}

	// 1. 配置加载（env 已在 config.Load 内覆盖文件）
	cfg, err := config.Load(flags.configPath)
	if err != nil {
		fmt.Fprintf(stderr, "❌ 配置加载失败: %v\n", err)
		return 1
	}
	if flags.configPath != "" {
		printlnHuman(stdout, fmt.Sprintf("⚙️  已加载配置：%s", flags.configPath), flags.jsonOut)
	}

	// 2. 会话加载（可选），并据此解析知识库/工具（flag > 会话文件 > 配置文件）
	var loaded *session.Session
	if flags.loadPath != "" {
		loaded, err = session.Load(flags.loadPath)
		if err != nil {
			fmt.Fprintf(stderr, "❌ 会话加载失败: %v\n", err)
			return 1
		}
		printlnHuman(stdout, fmt.Sprintf("📂 已加载会话：%s（历史 %d 条）", flags.loadPath, len(loaded.History)), flags.jsonOut)
	}
	resolveKnowledgeDir(flags, cfg, loaded)

	// 3. Provider 选择
	provider, ok := buildProvider(flags, cfg, stdout, stderr)
	if !ok {
		return 1
	}

	// 4. 知识库 / 工具 / Agent 装配
	var kb *knowledge.Store
	if flags.knowledgeDir != "" {
		kb, err = knowledge.Load(flags.knowledgeDir)
		if err != nil {
			fmt.Fprintf(stderr, "❌ 知识库加载失败: %v\n", err)
			return 1
		}
		printlnHuman(stdout, fmt.Sprintf("📚 知识库已加载：%s（%d 个段落）", kb.Dir(), kb.Count()), flags.jsonOut)
	}
	reg := buildRegistry(flags, stderr)

	a := agent.New(agent.Options{
		Provider:     provider,
		SystemPrompt: agent.DefaultSystemPrompt,
		Knowledge:    kb,
		Tools:        reg,
		MaxHistory:   flags.historyLimit,
	})
	if loaded != nil {
		a.LoadHistory(loaded.History)
	}

	// 5. 单轮 --ask：问完即出，不进入 REPL
	if flags.oneShot != "" {
		r := askOnce(a, flags.oneShot)
		emitTurn(r, flags.jsonOut, stdout, stderr)
		saveSession(a, flags, stdout, stderr)
		return 0
	}

	// 6. REPL（stdin 循环）
	printlnHuman(stdout, "", flags.jsonOut)
	printlnHuman(stdout, "=== 隆中对 · 孔明军师 ===", flags.jsonOut)
	printlnHuman(stdout, "主公有何要事相询？（输入 exit 退出）", flags.jsonOut)
	mode := "一问一答（无历史）"
	if flags.interactive {
		mode = "多轮交互（内存历史）"
	}
	printlnHuman(stdout, fmt.Sprintf("💬 模式：%s", mode), flags.jsonOut)

	scanner := bufio.NewScanner(stdin)
	var turns []turnResult // JSON 模式：收集整场会话
	for {
		if !flags.jsonOut {
			fmt.Fprint(stdout, "主公> ")
		}
		if !scanner.Scan() {
			break
		}
		question := strings.TrimSpace(scanner.Text())
		if question == "" {
			continue
		}
		if question == "exit" || question == "quit" || question == "退出" {
			printlnHuman(stdout, "亮告退。后会有期！", flags.jsonOut)
			break
		}
		if !flags.interactive {
			a.Reset() // 非交互：每轮不带历史，避免上下文串扰
		}
		r := askOnce(a, question)
		if r != nil {
			emitTurn(r, flags.jsonOut, stdout, stderr)
			if flags.jsonOut {
				turns = append(turns, *r)
			}
		}
	}

	saveSession(a, flags, stdout, stderr)

	// JSON 模式：多轮结束输出 session 汇总
	if flags.jsonOut && len(turns) > 0 {
		emitSession(turns, stdout)
	}
	return 0
}

// resolveKnowledgeDir 知识库目录优先级：flag > 会话文件 > 配置文件；工具同理
func resolveKnowledgeDir(flags *runFlags, cfg *config.Config, loaded *session.Session) {
	if flags.knowledgeDir == "" {
		if loaded != nil && loaded.KnowledgeDir != "" {
			flags.knowledgeDir = loaded.KnowledgeDir
		} else if cfg.KnowledgeDir != "" {
			flags.knowledgeDir = cfg.KnowledgeDir
		}
	}
	if flags.toolName == "" && cfg.Tool != "" {
		flags.toolName = cfg.Tool
	}
}

// buildProvider 选择 Provider：--mock 用 Mock，否则从配置（env 已合并）构造 OpenAI 兼容实现
func buildProvider(flags *runFlags, cfg *config.Config, stdout, stderr io.Writer) (llm.Provider, bool) {
	if flags.mock {
		printlnHuman(stdout, "⚙️  离线演示模式（Mock Provider）", flags.jsonOut)
		return &llm.MockProvider{}, true
	}
	if cfg.APIKey == "" {
		fmt.Fprintln(stderr, "❌ 未配置 LLM API Key")
		fmt.Fprintln(stderr, "")
		fmt.Fprintln(stderr, "请任选一家 OpenAI 兼容服务配置：")
		fmt.Fprintln(stderr, "  1. 环境变量：export KONGMING_API_KEY=sk-xxx")
		fmt.Fprintln(stderr, "  2. 配置文件：--config config.example.yaml（api_key 字段）")
		fmt.Fprintln(stderr, "")
		fmt.Fprintln(stderr, "或使用离线演示：kongming --mock")
		return nil, false
	}
	p := llm.NewOpenAIProvider(cfg.APIKey, cfg.BaseURL, cfg.Model)
	printlnHuman(stdout, fmt.Sprintf("⚙️  Provider: %s | Model: %s", p.Name(), displayModel(cfg.Model)), flags.jsonOut)
	return p, true
}

// displayModel 显示模型名（空则回退内置默认）
func displayModel(model string) string {
	if model == "" {
		return "gpt-4o-mini"
	}
	return model
}

// buildRegistry 按 --tool 组装工具注册表；当前仅支持 calc
func buildRegistry(flags *runFlags, stderr io.Writer) *tools.Registry {
	switch flags.toolName {
	case "calc":
		return tools.NewRegistry(tools.NewCalculator())
	case "":
		return nil
	default:
		fmt.Fprintf(stderr, "⚠️  未知工具：%s（当前支持：calc），已忽略\n", flags.toolName)
		return nil
	}
}

// askOnce 单轮问计，组装 JSON turn 对象
func askOnce(a *agent.Agent, question string) *turnResult {
	res := &turnResult{
		Type:     "turn",
		Question: question,
		General:  "诸葛亮",
	}
	reply, err := a.Ask(context.Background(), question)
	if err != nil {
		res.Success = false
		res.Error = err.Error()
		return res
	}
	res.Success = true
	res.Answer = reply.Answer
	res.Model = reply.Model
	res.Turns = reply.Turns
	res.Tool = reply.ToolUsed
	res.Knowledge = reply.Knowledge
	return res
}

// saveSession 退出时把当前会话（history + knowledge 配置）写入 JSON
func saveSession(a *agent.Agent, flags *runFlags, stdout, stderr io.Writer) {
	if flags.savePath == "" {
		return
	}
	hist := a.History()
	knowledgeDir := a.KnowledgeDir()
	if len(hist) == 0 && knowledgeDir == "" {
		printlnHuman(stdout, "⚠️  无对话历史，跳过保存（--save 需要至少一轮对话）", flags.jsonOut)
		return
	}
	ss := session.New(knowledgeDir, hist)
	if err := ss.Save(flags.savePath); err != nil {
		fmt.Fprintf(stderr, "❌ 会话保存失败: %v\n", err)
		return
	}
	printlnHuman(stdout, fmt.Sprintf("💾 会话已保存：%s（历史 %d 条）", flags.savePath, len(hist)), flags.jsonOut)
}

// ===== JSON / 人类输出 =====

// turnResult 单轮对话的结构化结果（JSON 输出契约，勿改字段）
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

// emitTurn 输出单轮结果：JSON 模式打印对象到 stdout，人类模式打印战报
func emitTurn(r *turnResult, jsonOut bool, stdout, stderr io.Writer) {
	if r == nil {
		return
	}
	if jsonOut {
		b, err := json.Marshal(r)
		if err != nil {
			fmt.Fprintf(stderr, `{"type":"error","message":%q}`+"\n", err.Error())
			return
		}
		fmt.Fprintln(stdout, string(b))
		return
	}

	if !r.Success {
		fmt.Fprintf(stderr, "❌ %s：%s\n", r.General, r.Error)
		return
	}
	if r.Tool != "" {
		fmt.Fprintf(stdout, "🛠️  %s（工具：%s）：\n", r.General, r.Tool)
	} else {
		fmt.Fprintf(stdout, "🧠 %s：\n", r.General)
	}
	fmt.Fprintln(stdout, r.Answer)
	if len(r.Knowledge) > 0 {
		fmt.Fprintf(stdout, "📚 参考知识：%s\n", strings.Join(r.Knowledge, "、"))
	}
	if r.Model != "" {
		fmt.Fprintf(stdout, "（模型：%s | 消息 %d 条）\n", r.Model, r.Turns)
	}
}

// emitSession 输出整场会话汇总（JSON 模式，多轮结束时）
func emitSession(turns []turnResult, stdout io.Writer) {
	summary := sessionResult{
		Type:       "session",
		TotalTurns: len(turns),
		Questions:  make([]string, 0, len(turns)),
		Turns:      turns,
	}
	for _, r := range turns {
		summary.Questions = append(summary.Questions, r.Question)
	}
	b, err := json.Marshal(summary)
	if err != nil {
		fmt.Fprintf(stdout, `{"type":"error","message":%q}`+"\n", err.Error())
		return
	}
	fmt.Fprintln(stdout, string(b))
}

// printlnHuman 人类友好输出辅助：JSON 模式下跳过装饰行
func printlnHuman(w io.Writer, s string, jsonOut bool) {
	if !jsonOut && s != "" {
		fmt.Fprintln(w, s)
	}
}
