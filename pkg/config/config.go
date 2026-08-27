// 配置文件支持 - YAML/JSON 配置加载（环境变量优先）
// 运筹帷幄，谋定后动
//
// 优先级：命令行（--config 显式指定）→ 环境变量 → 配置文件 → 内置默认。
// 设计约束：配置文件仅承载 LLM 连接/知识库/默认工具等 demo 级设置，
// 不做运行时热更新、不做密钥加密（API Key 建议仍走环境变量）。

package config

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config 配置结构
type Config struct {
	// LLM 连接
	APIKey   string `json:"api_key" yaml:"api_key"`   // OpenAI 兼容 API Key
	BaseURL  string `json:"base_url" yaml:"base_url"` // Base URL
	Model    string `json:"model" yaml:"model"`       // 模型名
	Provider string `json:"provider" yaml:"provider"` // 指标标签
	// 知识库（轻量 RAG）
	KnowledgeDir string `json:"knowledge_dir" yaml:"knowledge_dir"` // 本地知识库目录
	// 默认工具
	Tool string `json:"tool" yaml:"tool"` // 默认启用工具（如 calc）
}

// Load 读取配置文件（YAML 或 JSON，按扩展名自动识别），环境变量优先。
// 返回合并后的配置；未指定文件时返回空配置（环境变量照常生效）。
func Load(path string) (*Config, error) {
	cfg := &Config{}
	if path != "" {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("读取配置文件失败: %w", err)
		}
		if err := parse(data, path, cfg); err != nil {
			return nil, err
		}
	}
	// 环境变量优先覆盖
	applyEnv(cfg)
	return cfg, nil
}

// parse 按扩展名解析：.json → JSON，其余按 YAML（含 .yaml/.yml/无扩展名）
func parse(data []byte, path string, cfg *Config) error {
	lower := strings.ToLower(path)
	if strings.HasSuffix(lower, ".json") {
		if err := json.Unmarshal(data, cfg); err != nil {
			return fmt.Errorf("解析 JSON 配置失败: %w", err)
		}
		return nil
	}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return fmt.Errorf("解析 YAML 配置失败: %w", err)
	}
	return nil
}

// applyEnv 用环境变量覆盖配置值（环境变量优先）
func applyEnv(cfg *Config) {
	if v := os.Getenv("KONGMING_API_KEY"); v != "" {
		cfg.APIKey = v
	}
	if v := os.Getenv("KONGMING_BASE_URL"); v != "" {
		cfg.BaseURL = v
	}
	if v := os.Getenv("KONGMING_MODEL"); v != "" {
		cfg.Model = v
	}
	if v := os.Getenv("KONGMING_PROVIDER"); v != "" {
		cfg.Provider = v
	}
}
