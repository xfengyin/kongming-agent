// 诸葛亮 - LLM 驱动的军师将领
// 运筹帷幄之中，决胜千里之外

package generals

import (
	"context"
	"time"

	"github.com/zhuge/kongming/pkg/core"
	"github.com/zhuge/kongming/pkg/llm"
)

// GeneralKongMing 诸葛亮类型（军师 - LLM 驱动）
const GeneralKongMing GeneralType = "kongming"

// KongMingSystemPrompt 诸葛亮人设（锦囊）
const KongMingSystemPrompt = `你是诸葛亮（字孔明），蜀汉丞相，智慧的化身。
你以《隆中对》闻名：审时度势、分析天下大势、提出可行战略。
请以简洁、务实、略带文言风骨的中文回答主公的咨询。
分析要点：1) 局势判断 2) 关键矛盾 3) 行动建议。`

// KongMingHandler 诸葛亮处理器：调用真实 LLM 提供者
type KongMingHandler struct {
	Provider llm.Provider
}

// Execute 执行军令：将主公问题交由 LLM 回答
func (h *KongMingHandler) Execute(ctx context.Context, order *core.MilitaryOrder) (*core.GeneralReport, error) {
	start := time.Now()

	if h.Provider == nil {
		return &core.GeneralReport{
			GeneralID:   "kongming",
			GeneralName: "诸葛亮",
			Success:     false,
			Message:     "军师未配置 LLM Provider，请设置 KONGMING_API_KEY 环境变量",
			Data: map[string]interface{}{
				"hint":       "export KONGMING_API_KEY=sk-xxx（DeepSeek/OpenAI/通义 任一 OpenAI 兼容 Key）",
				"elapsed_ms": float64(time.Since(start).Milliseconds()),
			},
		}, nil
	}

	question := order.Description
	if q, ok := order.Context["question"].(string); ok && q != "" {
		question = q
	}

	resp, err := h.Provider.Chat(ctx, llm.ChatRequest{
		Model: "",
		Messages: []llm.Message{
			{Role: llm.RoleSystem, Content: KongMingSystemPrompt},
			{Role: llm.RoleUser, Content: question},
		},
		Temperature: 0.7,
		MaxTokens:   1024,
	})
	if err != nil {
		return &core.GeneralReport{
			GeneralID:   "kongming",
			GeneralName: "诸葛亮",
			Success:     false,
			Message:     "军师思虑受阻：" + err.Error(),
			Data: map[string]interface{}{
				"error":      err.Error(),
				"elapsed_ms": float64(time.Since(start).Milliseconds()),
			},
		}, nil
	}

	return &core.GeneralReport{
		GeneralID:   "kongming",
		GeneralName: "诸葛亮",
		Success:     true,
		Message:     "亮已思得一计",
		Data: map[string]interface{}{
			"answer":     resp.Content,
			"model":      resp.Model,
			"provider":   h.Provider.Name(),
			"elapsed_ms": float64(time.Since(start).Milliseconds()),
		},
	}, nil
}

// NewWuHuPoolWithLLM 创建五虎将池并附加军师诸葛亮（LLM 驱动）。
// provider 为 nil 时诸葛亮仍注册，但执行会返回引导配置的提示。
func NewWuHuPoolWithLLM(provider llm.Provider) *WuHuPool {
	pool := NewWuHuPool()

	pool.Register(&General{
		ID:          "kongming",
		Name:        "诸葛亮",
		Type:        GeneralKongMing,
		Title:       "军师",
		Description: "运筹帷幄的 LLM 军师，可分析天下大势、给出战略建议",
		Skills:      []string{"llm", "strategy", "consulting", "planning"},
		Traits:      map[string]interface{}{"wisdom": 0.99, "strategy": 0.99},
		Stats:       GeneralStats{},
		State:       GeneralIdle,
		CreatedAt:   time.Now(),
	})
	pool.SetLLMProvider(provider)
	return pool
}

// SetLLMProvider 为已注册的诸葛亮附加/更换 LLM Provider
func (p *WuHuPool) SetLLMProvider(provider llm.Provider) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.handlers[GeneralKongMing] = &KongMingHandler{Provider: provider}
}
