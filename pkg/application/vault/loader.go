// Package vault loader.go 提供 LoadFromDir：从目录加载锦囊定义文件。
//
// 支持的文件格式：
//   - *.json：使用 encoding/json 解析
//   - *.yaml / *.yml：纯字符串解析（id/name/type/tags 行），不引入 yaml 依赖
//   - 其它后缀 / 子目录：跳过
//
// 注册策略：解析成功 → 转成 *model.Jinnang → 调 RegisterSkill。
// 默认 handler 用 echoHandler（占位），业务方可在加载后用 RegisterSkill 覆盖。
//
// 设计原则：
//   - 失败快速：任一文件解析失败 → 整个 LoadFromDir 失败（不静默吞错）
//   - 幂等：相同目录二次加载，覆盖已存在的 ID（不报重）
//   - 防御：目录不存在 → 返回 error
package vault

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"go.uber.org/zap"

	"github.com/zhuge/kongming/pkg/domain/model"
)

// jinnangFile 是从 JSON 文件反序列化的临时结构，与 model.Jinnang 解耦。
// 这样做的好处：文件 schema 与运行期实体可独立演进（开闭原则）。
type jinnangFile struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	Type        string         `json:"type"`
	Description string         `json:"description"`
	Version     string         `json:"version"`
	Tags        []string       `json:"tags"`
	Config      map[string]any `json:"config"`
}

// LoadFromDir 扫描 dir 下的 *.json / *.yaml / *.yml 文件，批量注册到 vault。
//
// 行为契约：
//   - 目录不存在 → error
//   - 任一文件解析失败 → error（包含文件名 + 行号/字段名）
//   - 成功注册 0 个或 N 个，返回 nil
//   - ctx 当前未用于取消（解析同步），但保留参数对齐 port.Vault 接口
func (v *Vault) LoadFromDir(ctx context.Context, dir string) error {
	// 防御：ctx 为 nil 时退回 Background（不让 nil 透传到下游）。
	if ctx == nil {
		ctx = context.Background()
	}
	_ = ctx

	// 1) 目录存在性校验。
	info, err := os.Stat(dir)
	if err != nil {
		return fmt.Errorf("vault.LoadFromDir: 访问目录失败: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("vault.LoadFromDir: %s 不是目录", dir)
	}

	// 2) 遍历目录（按文件名排序，便于可观测/幂等）。
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("vault.LoadFromDir: 读取目录失败: %w", err)
	}

	loaded := 0
	for _, e := range entries {
		// 跳过子目录与隐藏文件。
		if e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		path := filepath.Join(dir, e.Name())
		ext := strings.ToLower(filepath.Ext(e.Name()))

		switch ext {
		case ".json":
			if err := v.loadJSONFile(path); err != nil {
				return fmt.Errorf("load %s: %w", path, err)
			}
			loaded++
		case ".yaml", ".yml":
			if err := v.loadYAMLFile(path); err != nil {
				return fmt.Errorf("load %s: %w", path, err)
			}
			loaded++
		default:
			// 其它后缀（.md, .txt, ...）静默跳过。
			v.logger.Debug("vault.LoadFromDir: 跳过非锦囊文件",
				zap.String("file", e.Name()))
		}
	}

	v.logger.Info("vault.LoadFromDir: 加载完成",
		zap.String("dir", dir),
		zap.Int("loaded", loaded),
	)
	return nil
}

// loadJSONFile 解析单个 .json 文件并注册。
func (v *Vault) loadJSONFile(path string) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("读取文件: %w", err)
	}
	var jf jinnangFile
	if err := json.Unmarshal(raw, &jf); err != nil {
		return fmt.Errorf("json 解析失败: %w", err)
	}
	return v.registerFromFile(jf, path)
}

// loadYAMLFile 用纯字符串解析单个 .yaml/.yml 文件。
// 仅识别顶层 id/name/type/version/description/tags 行（每行一项）。
// 注释以 # 开头会被忽略。复杂嵌套结构（config: {...}）不被支持。
//
// 选择不引入 yaml 库的理由：
//   - 减小依赖体积（任务规范明确要求「不引入新依赖」）
//   - 内置锦囊定义文件保持极简（id/name/type/tags/config 四字段）
func (v *Vault) loadYAMLFile(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("读取文件: %w", err)
	}
	defer f.Close()

	jf := jinnangFile{}
	scanner := bufio.NewScanner(f)
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := strings.TrimSpace(scanner.Text())
		// 跳过空行 / 注释。
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		// 仅识别「key: value」形式。
		idx := strings.Index(line, ":")
		if idx <= 0 {
			continue
		}
		key := strings.TrimSpace(line[:idx])
		val := strings.TrimSpace(line[idx+1:])
		// 去掉可选引号。
		val = strings.Trim(val, `"'`)

		switch key {
		case "id":
			jf.ID = val
		case "name":
			jf.Name = val
		case "type":
			jf.Type = val
		case "version":
			jf.Version = val
		case "description":
			jf.Description = val
		case "tags":
			// 极简：[a, b, c] → ["a","b","c"]
			val = strings.Trim(val, "[]")
			if val == "" {
				continue
			}
			for _, t := range strings.Split(val, ",") {
				t = strings.TrimSpace(t)
				t = strings.Trim(t, `"'`)
				if t != "" {
					jf.Tags = append(jf.Tags, t)
				}
			}
		case "config":
			// 不解析嵌套；保留为 nil。
		default:
			// 其它键忽略（避免被陌生字段炸掉）。
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("扫描第 %d 行: %w", lineNo, err)
	}
	return v.registerFromFile(jf, path)
}

// registerFromFile 把 jinnangFile 转成 model.Jinnang 并注册。
// handler 默认注入 echoHandler（占位），调用方可在加载后用
// RegisterSkill 覆盖。
func (v *Vault) registerFromFile(jf jinnangFile, src string) error {
	if jf.ID == "" {
		return fmt.Errorf("缺少 id 字段")
	}
	if jf.Name == "" {
		jf.Name = jf.ID // 兜底：用 ID 充当 Name
	}
	// Type 缺省时回退到 skill。
	if jf.Type == "" {
		jf.Type = string(model.JinnangSkill)
	}
	if jf.Version == "" {
		jf.Version = "0.0.0"
	}
	j := &model.Jinnang{
		ID:          jf.ID,
		Name:        jf.Name,
		Type:        model.JinnangType(jf.Type),
		Description: jf.Description,
		Version:     jf.Version,
		Tags:        jf.Tags,
		Config:      jf.Config,
	}
	// 占位 handler：业务方可在加载完成后用 RegisterSkill 覆盖。
	if err := v.RegisterSkill(j, &echoHandler{}); err != nil {
		return fmt.Errorf("注册失败 (id=%s, src=%s): %w", jf.ID, src, err)
	}
	return nil
}
