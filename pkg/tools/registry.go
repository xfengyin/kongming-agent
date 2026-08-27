// 工具注册表 - 统一入口调度各工具
// 按注册顺序 first-match-wins，命中的首个工具接管问题。

package tools

import "context"

// Tool 表示一个可被 Agent 调用的工具。
// Try 判断 question 是否命中该工具；命中返回 handled=true 及执行结果。
// 命中但执行失败返回 handled=true, err!=nil（由调用方决定是否回落 LLM）。
type Tool interface {
	Name() string
	Try(ctx context.Context, question string) (handled bool, output string, err error)
}

// Registry 工具注册表
type Registry struct {
	tools []Tool
}

// NewRegistry 创建工具注册表
func NewRegistry(tools ...Tool) *Registry {
	return &Registry{tools: tools}
}

// Add 追加一个工具
func (r *Registry) Add(t Tool) { r.tools = append(r.tools, t) }

// Try 依次尝试各工具，返回首个命中的结果（含命中但失败的场景）。
// 无工具命中返回 handled=false。
func (r *Registry) Try(ctx context.Context, question string) (handled bool, toolName, output string, err error) {
	for _, t := range r.tools {
		handled, output, err := t.Try(ctx, question)
		if handled {
			return true, t.Name(), output, err
		}
	}
	return false, "", "", nil
}
