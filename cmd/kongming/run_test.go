// CLI 集成测试 - 直接调用 run()，不起子进程
// 覆盖：单轮/交互/JSON 契约/会话往返/工具/配置/历史截断/错误路径。

package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// runCLI 便捷封装：喂 stdin，返回 (exitCode, stdout, stderr)
func runCLI(t *testing.T, stdin string, args ...string) (int, string, string) {
	t.Helper()
	var out, errBuf bytes.Buffer
	code := run(args, strings.NewReader(stdin), &out, &errBuf)
	return code, out.String(), errBuf.String()
}

// parseTurns 解析 stdout 中所有 type:"turn" 对象
func parseTurns(t *testing.T, stdout string) []turnResult {
	t.Helper()
	var turns []turnResult
	for _, line := range strings.Split(stdout, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var obj map[string]interface{}
		if err := json.Unmarshal([]byte(line), &obj); err != nil {
			t.Fatalf("stdout 含非法 JSON: %q", line)
		}
		if obj["type"] == "turn" {
			var tr turnResult
			if err := json.Unmarshal([]byte(line), &tr); err != nil {
				t.Fatalf("解析 turn 失败: %v", err)
			}
			turns = append(turns, tr)
		}
	}
	return turns
}

func TestRunMockAskJSON(t *testing.T) {
	code, stdout, _ := runCLI(t, "", "--mock", "--json", "--ask", "天下大势如何？")
	if code != 0 {
		t.Fatalf("退出码应为 0，实际 %d", code)
	}
	turns := parseTurns(t, stdout)
	if len(turns) != 1 {
		t.Fatalf("应恰好输出一个 turn 对象，实际 %d 个", len(turns))
	}
	tr := turns[0]
	if tr.Type != "turn" || tr.Question != "天下大势如何？" || !tr.Success {
		t.Errorf("turn 字段错误: %+v", tr)
	}
	if tr.Answer == "" {
		t.Errorf("answer 不应为空")
	}
	if tr.Turns != 2 {
		t.Errorf("单轮 turns 应为 2，实际 %d", tr.Turns)
	}
}

func TestRunMockAskHuman(t *testing.T) {
	code, stdout, _ := runCLI(t, "", "--mock", "--ask", "天下大势如何？")
	if code != 0 {
		t.Fatalf("退出码应为 0，实际 %d", code)
	}
	if !strings.Contains(stdout, "天下大势如何") {
		t.Errorf("人类输出应包含问题，实际: %q", stdout)
	}
	if !strings.Contains(stdout, "主公>") && strings.Contains(stdout, "=== 隆中对") {
		// REPL 头应在非 ask 模式出现；--ask 直接问完即出，无 REPL 头
		t.Errorf("--ask 不应输出 REPL 头")
	}
}

func TestRunInteractiveJSONSession(t *testing.T) {
	code, stdout, _ := runCLI(t, "第一问\n第二问\nexit\n", "--mock", "--interactive", "--json")
	if code != 0 {
		t.Fatalf("退出码应为 0，实际 %d", code)
	}
	turns := parseTurns(t, stdout)
	if len(turns) != 2 {
		t.Fatalf("应输出 2 个 turn，实际 %d", len(turns))
	}
	// 第二轮的 turns 应反映多轮历史（system + 历史2 + user = 4）
	if turns[1].Turns != 4 {
		t.Errorf("第二轮 turns 应为 4，实际 %d", turns[1].Turns)
	}
	// 最后应有 session 汇总
	if !strings.Contains(stdout, `"type":"session"`) {
		t.Errorf("交互结束应输出 session 汇总，实际: %q", stdout)
	}
	if !strings.Contains(stdout, `"total_turns":2`) {
		t.Errorf("session total_turns 应为 2，实际: %q", stdout)
	}
}

func TestRunInteractiveNonJSON(t *testing.T) {
	code, stdout, _ := runCLI(t, "你好\nexit\n", "--mock", "--interactive")
	if code != 0 {
		t.Fatalf("退出码应为 0，实际 %d", code)
	}
	if !strings.Contains(stdout, "=== 隆中对") {
		t.Errorf("交互模式应输出标题，实际: %q", stdout)
	}
	if strings.Contains(stdout, `"type":"turn"`) {
		t.Errorf("非 JSON 模式不应输出 JSON 对象")
	}
}

func TestRunToolCalc(t *testing.T) {
	code, stdout, _ := runCLI(t, "", "--mock", "--json", "--tool", "calc", "--ask", "计算 123*456")
	if code != 0 {
		t.Fatalf("退出码应为 0，实际 %d", code)
	}
	turns := parseTurns(t, stdout)
	if len(turns) != 1 {
		t.Fatalf("应输出 1 个 turn，实际 %d", len(turns))
	}
	tr := turns[0]
	if tr.Tool != "calc" {
		t.Errorf("应命中 calc 工具，实际 %q", tr.Tool)
	}
	if !strings.Contains(tr.Answer, "56088") {
		t.Errorf("工具结果应包含 56088，实际 %q", tr.Answer)
	}
}

func TestRunSaveLoadRoundTrip(t *testing.T) {
	savePath := filepath.Join(t.TempDir(), "session.json")

	// 交互一轮 + exit → 保存
	code, _, stderr := runCLI(t, "第一问\n退出\n", "--mock", "--interactive", "--save", savePath)
	if code != 0 {
		t.Fatalf("保存会话退出码应为 0，实际 %d（stderr=%s）", code, stderr)
	}
	if _, err := os.Stat(savePath); err != nil {
		t.Fatalf("会话文件未生成: %v", err)
	}

	// 加载会话 + 单轮问计：历史应透传（system + 历史2 + user = 4）
	code, stdout, _ := runCLI(t, "", "--mock", "--json", "--load", savePath, "--ask", "第二问")
	if code != 0 {
		t.Fatalf("加载会话退出码应为 0，实际 %d", code)
	}
	turns := parseTurns(t, stdout)
	if len(turns) != 1 {
		t.Fatalf("应输出 1 个 turn，实际 %d", len(turns))
	}
	if turns[0].Turns != 4 {
		t.Errorf("加载历史后 turns 应为 4（system+历史2+user），实际 %d", turns[0].Turns)
	}
}

func TestRunSaveWithoutHistory(t *testing.T) {
	savePath := filepath.Join(t.TempDir(), "empty.json")
	code, _, _ := runCLI(t, "", "--mock", "--interactive", "--save", savePath)
	if code != 0 {
		t.Fatalf("退出码应为 0，实际 %d", code)
	}
	if _, err := os.Stat(savePath); err == nil {
		t.Errorf("无对话历史不应生成会话文件")
	}
}

func TestRunNoAPIKey(t *testing.T) {
	// 无 --mock 且未配置 Key → 引导错误，退出码 1
	code, _, stderr := runCLI(t, "", "--ask", "你好")
	if code != 1 {
		t.Fatalf("未配置 Key 退出码应为 1，实际 %d", code)
	}
	if !strings.Contains(stderr, "KONGMING_API_KEY") {
		t.Errorf("错误应引导配置 Key，实际: %q", stderr)
	}
}

func TestRunVersion(t *testing.T) {
	code, stdout, _ := runCLI(t, "", "--version")
	if code != 0 {
		t.Fatalf("退出码应为 0，实际 %d", code)
	}
	if !strings.Contains(stdout, version) {
		t.Errorf("版本输出应含 %s，实际 %q", version, stdout)
	}
}

func TestRunUnknownFlag(t *testing.T) {
	code, _, _ := runCLI(t, "", "--no-such-flag")
	if code != 2 {
		t.Fatalf("未知 flag 退出码应为 2，实际 %d", code)
	}
}

func TestRunHistoryLimit(t *testing.T) {
	// --history-limit 1：前 3 轮后历史截断，后续轮 turns 恒为 4（system+历史2+user）
	code, stdout, _ := runCLI(t, "一\n二\n三\n四\nexit\n", "--mock", "--interactive", "--json", "--history-limit", "1")
	if code != 0 {
		t.Fatalf("退出码应为 0，实际 %d", code)
	}
	turns := parseTurns(t, stdout)
	if len(turns) != 4 {
		t.Fatalf("应输出 4 个 turn，实际 %d", len(turns))
	}
	// 第一轮无历史 → 2；后续三轮截断为 2 条历史 → 4
	if turns[0].Turns != 2 {
		t.Errorf("第一轮 turns 应为 2，实际 %d", turns[0].Turns)
	}
	for i := 1; i < 4; i++ {
		if turns[i].Turns != 4 {
			t.Errorf("第 %d 轮 turns 应为 4（截断后），实际 %d", i+1, turns[i].Turns)
		}
	}
}

func TestRunConfigFile(t *testing.T) {
	// 配置文件：api_key + tool + knowledge_dir（指向临时知识库）
	kbDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(kbDir, "sanguo.md"), []byte("# 空城计\n\n诸葛亮大开城门，焚香抚琴。\n"), 0o644); err != nil {
		t.Fatalf("写知识文件失败: %v", err)
	}
	cfgPath := filepath.Join(t.TempDir(), "kongming.yaml")
	kbSlash := filepath.ToSlash(kbDir) // YAML 双引号内反斜杠是转义符，改用正斜杠
	cfgYAML := "api_key: \"sk-test\"\nbase_url: \"https://example.com/v1\"\nmodel: \"test-model\"\nknowledge_dir: \"" + kbSlash + "\"\ntool: \"calc\"\n"
	if err := os.WriteFile(cfgPath, []byte(cfgYAML), 0o644); err != nil {
		t.Fatalf("写配置文件失败: %v", err)
	}

	// --mock + --config：知识库与工具均来自配置
	code, stdout, stderr := runCLI(t, "", "--mock", "--json", "--config", cfgPath, "--ask", "如何用空城计退敌？")
	if code != 0 {
		t.Fatalf("退出码应为 0，实际 %d（stderr=%s）", code, stderr)
	}
	turns := parseTurns(t, stdout)
	if len(turns) != 1 {
		t.Fatalf("应输出 1 个 turn，实际 %d", len(turns))
	}
	if len(turns[0].Knowledge) == 0 {
		t.Errorf("知识库应从配置加载并命中，实际无 Knowledge")
	}
	if turns[0].Turns != 3 {
		t.Errorf("知识注入后 turns 应为 3（人设+知识+问题），实际 %d", turns[0].Turns)
	}
}

func TestRunUnknownTool(t *testing.T) {
	code, stdout, stderr := runCLI(t, "", "--mock", "--tool", "hack", "--ask", "你好")
	if code != 0 {
		t.Fatalf("退出码应为 0，实际 %d", code)
	}
	if !strings.Contains(stderr, "未知工具") {
		t.Errorf("应警告未知工具，实际 stderr=%q", stderr)
	}
	if !strings.Contains(stdout, "你好") {
		t.Errorf("未知工具应忽略并正常对话，实际 stdout=%q", stdout)
	}
}
