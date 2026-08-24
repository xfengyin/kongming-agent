// 配置文件测试

package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const yamlSample = `
api_key: "sk-from-yaml"
base_url: "https://api.deepseek.com/v1"
model: "deepseek-chat"
provider: "deepseek"
knowledge_dir: "./knowledge"
tool: "calc"
`

const jsonSample = `{
  "api_key": "sk-from-json",
  "base_url": "https://api.openai.com/v1",
  "model": "gpt-4o-mini",
  "provider": "openai",
  "knowledge_dir": "./kb",
  "tool": ""
}`

func writeFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatalf("写文件失败: %v", err)
	}
	return p
}

func TestLoadYAML(t *testing.T) {
	path := writeFile(t, t.TempDir(), "config.yaml", yamlSample)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load 失败: %v", err)
	}
	if cfg.APIKey != "sk-from-yaml" || cfg.BaseURL != "https://api.deepseek.com/v1" {
		t.Errorf("YAML 字段解析错误: %+v", cfg)
	}
	if cfg.Model != "deepseek-chat" || cfg.Provider != "deepseek" {
		t.Errorf("YAML 字段解析错误: %+v", cfg)
	}
	if cfg.KnowledgeDir != "./knowledge" || cfg.Tool != "calc" {
		t.Errorf("知识库/工具字段错误: %+v", cfg)
	}
}

func TestLoadJSON(t *testing.T) {
	path := writeFile(t, t.TempDir(), "config.json", jsonSample)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load 失败: %v", err)
	}
	if cfg.APIKey != "sk-from-json" || cfg.Model != "gpt-4o-mini" {
		t.Errorf("JSON 字段解析错误: %+v", cfg)
	}
	if cfg.KnowledgeDir != "./kb" {
		t.Errorf("知识库字段错误: %+v", cfg)
	}
}

func TestLoadEnvTakesPriority(t *testing.T) {
	path := writeFile(t, t.TempDir(), "config.yaml", yamlSample)
	t.Setenv("KONGMING_API_KEY", "sk-from-env")
	t.Setenv("KONGMING_MODEL", "env-model")
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load 失败: %v", err)
	}
	if cfg.APIKey != "sk-from-env" {
		t.Errorf("环境变量应优先于配置文件: %s", cfg.APIKey)
	}
	if cfg.Model != "env-model" {
		t.Errorf("环境变量应优先于配置文件: %s", cfg.Model)
	}
	// 未设置环境变量的字段仍取配置文件
	if cfg.BaseURL != "https://api.deepseek.com/v1" {
		t.Errorf("未覆盖字段应取配置值: %s", cfg.BaseURL)
	}
}

func TestLoadMissingFile(t *testing.T) {
	_, err := Load(filepath.Join(t.TempDir(), "no-such.yaml"))
	if err == nil {
		t.Fatal("缺失文件应报错")
	}
	if !strings.Contains(err.Error(), "读取配置文件失败") {
		t.Errorf("错误信息不明确: %v", err)
	}
}

func TestLoadInvalidFormat(t *testing.T) {
	// 非法 YAML
	p1 := writeFile(t, t.TempDir(), "bad.yaml", "::: not : yaml :::")
	if _, err := Load(p1); err == nil {
		t.Error("非法 YAML 应报错")
	}
	// 非法 JSON
	p2 := writeFile(t, t.TempDir(), "bad.json", "{not json}")
	if _, err := Load(p2); err == nil {
		t.Error("非法 JSON 应报错")
	}
}

func TestLoadEmptyPath(t *testing.T) {
	cfg, err := Load("")
	if err != nil {
		t.Fatalf("空路径 Load 失败: %v", err)
	}
	if cfg.APIKey != "" {
		t.Errorf("空路径应返回空配置: %+v", cfg)
	}
}

func TestApplySetsEnv(t *testing.T) {
	t.Setenv("KONGMING_API_KEY", "") // 确保环境变量为空
	cfg := &Config{
		APIKey:   "sk-from-config",
		BaseURL:  "https://api.qwen.com/v1",
		Model:    "qwen-plus",
		Provider: "qwen",
	}
	cfg.Apply()
	if os.Getenv("KONGMING_API_KEY") != "sk-from-config" {
		t.Errorf("Apply 应写入环境变量: %s", os.Getenv("KONGMING_API_KEY"))
	}
	if os.Getenv("KONGMING_MODEL") != "qwen-plus" {
		t.Errorf("Apply 应写入环境变量: %s", os.Getenv("KONGMING_MODEL"))
	}
}

func TestApplyDoesNotOverwriteEnv(t *testing.T) {
	t.Setenv("KONGMING_API_KEY", "sk-already-set")
	cfg := &Config{APIKey: "sk-from-config"}
	cfg.Apply()
	if os.Getenv("KONGMING_API_KEY") != "sk-already-set" {
		t.Errorf("Apply 不应覆盖已有环境变量: %s", os.Getenv("KONGMING_API_KEY"))
	}
}
