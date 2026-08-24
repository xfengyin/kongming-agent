// 会话持久化 - 保存/恢复多轮对话
// 好记性不如烂笔头
//
// 设计约束：
//   - 纯 JSON 文件，零外部依赖；
//   - 保存 history（messages 数组）与 knowledge 配置；
//   - 损坏文件/非法 JSON 返回明确错误，不静默吞掉。

package session

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/zhuge/kongming/pkg/llm"
)

// Version 当前会话文件格式版本
const Version = 1

// Session 可持久化的会话
type Session struct {
	Version      int           `json:"version"`
	CreatedAt    time.Time     `json:"created_at"`
	UpdatedAt    time.Time     `json:"updated_at"`
	KnowledgeDir string        `json:"knowledge_dir,omitempty"` // RAG 知识库目录配置
	History      []llm.Message `json:"history"`                 // 多轮对话历史
}

// New 创建新会话
func New(knowledgeDir string, history []llm.Message) *Session {
	now := time.Now()
	return &Session{
		Version:      Version,
		CreatedAt:    now,
		UpdatedAt:    now,
		KnowledgeDir: knowledgeDir,
		History:      history,
	}
}

// Save 将会话写入 JSON 文件（先写临时文件再 rename，避免半写文件）
func (s *Session) Save(path string) error {
	if s == nil {
		return fmt.Errorf("会话为空")
	}
	s.Version = Version
	s.UpdatedAt = time.Now()

	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化会话失败: %w", err)
	}

	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("写入会话文件失败: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("保存会话文件失败: %w", err)
	}
	return nil
}

// Load 从 JSON 文件加载会话
func Load(path string) (*Session, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("读取会话文件失败: %w", err)
	}

	var s Session
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("解析会话文件失败（文件可能已损坏）: %w", err)
	}
	if s.Version != Version {
		return nil, fmt.Errorf("会话文件版本不兼容: 期望 v%d，实际 v%d", Version, s.Version)
	}
	return &s, nil
}
