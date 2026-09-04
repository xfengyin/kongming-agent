// MCP 工具接入 - 把任意 MCP Server 的工具暴露为 Kongming 工具
// 他山之石，可以攻玉
//
// 设计约束：
//   - 通过 stdio 启动 MCP Server 子进程，协议细节委托 mark3labs/mcp-go；
//   - 命中规则为“用户问题包含工具名”（大小写不敏感），参数留空；
//     这适合 CLI 里的人工召唤场景；LLM 自主选工具请使用支持 function-calling
//     的 Provider 接入，而非本 pre-check 通道。

package tools

import (
	"context"
	"fmt"
	"strings"

	mcpclient "github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
)

// MCPServerTool 包装 MCP Server 上的一个工具。
type MCPServerTool struct {
	client *mcpclient.Client
	name   string
	desc   string
}

// Name 返回带 mcp: 前缀的工具名（避免与内置工具冲突）
func (m *MCPServerTool) Name() string { return "mcp:" + m.name }

// Description 工具描述
func (m *MCPServerTool) Description() string { return m.desc }

// Try 命中规则：问题文本包含工具名（大小写不敏感）。
func (m *MCPServerTool) Try(ctx context.Context, question string) (bool, string, error) {
	needle := strings.ToLower(m.name)
	if needle == "" || !strings.Contains(strings.ToLower(question), needle) {
		return false, "", nil
	}

	req := mcp.CallToolRequest{}
	req.Params.Name = m.name
	req.Params.Arguments = map[string]any{}
	result, err := m.client.CallTool(ctx, req)
	if err != nil {
		return true, "", fmt.Errorf("MCP 工具 %s 调用失败: %w", m.name, err)
	}
	if result.IsError {
		return true, "", fmt.Errorf("MCP 工具 %s 返回错误: %s", m.name, formatMCPResult(result))
	}
	return true, formatMCPResult(result), nil
}

// Close 关闭底层 MCP 客户端（结束子进程）
func (m *MCPServerTool) Close() error {
	if m.client == nil {
		return nil
	}
	return m.client.Close()
}

// LoadMCPTools 启动一个 stdio MCP Server，初始化后列出其全部工具。
// command 为可执行文件（如 npx），args 为参数（如 -y @modelcontextprotocol/server-filesystem /tmp）。
func LoadMCPTools(ctx context.Context, command string, args ...string) ([]*MCPServerTool, error) {
	if command == "" {
		return nil, nil
	}
	c, err := mcpclient.NewStdioMCPClient(command, []string{}, args...)
	if err != nil {
		return nil, fmt.Errorf("启动 MCP Server 失败: %w", err)
	}

	initReq := mcp.InitializeRequest{}
	initReq.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
	initReq.Params.ClientInfo = mcp.Implementation{Name: "kongming", Version: "0.9.0"}
	if _, err := c.Initialize(ctx, initReq); err != nil {
		_ = c.Close()
		return nil, fmt.Errorf("初始化 MCP Server 失败: %w", err)
	}

	toolsResult, err := c.ListTools(ctx, mcp.ListToolsRequest{})
	if err != nil {
		_ = c.Close()
		return nil, fmt.Errorf("列出 MCP 工具失败: %w", err)
	}

	out := make([]*MCPServerTool, 0, len(toolsResult.Tools))
	for _, t := range toolsResult.Tools {
		out = append(out, &MCPServerTool{client: c, name: t.Name, desc: t.Description})
	}
	return out, nil
}

// formatMCPResult 从 MCP 调用结果中提取文本内容。
func formatMCPResult(result *mcp.CallToolResult) string {
	if result == nil {
		return "(空结果)"
	}
	parts := make([]string, 0, len(result.Content))
	for _, c := range result.Content {
		switch v := c.(type) {
		case mcp.TextContent:
			parts = append(parts, v.Text)
		case *mcp.TextContent:
			parts = append(parts, v.Text)
		default:
			parts = append(parts, fmt.Sprintf("%v", c))
		}
	}
	if len(parts) == 0 {
		return "(无文本内容)"
	}
	return strings.Join(parts, "\n")
}
