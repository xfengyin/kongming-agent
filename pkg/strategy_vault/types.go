// 锦囊库 - 技能与工具管理系统
// 锦囊妙计，随时取用

package strategy_vault

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// JinnangType 锦囊类型
type JinnangType string

const (
	JinnangSkill  JinnangType = "skill"  // 技能锦囊
	JinnangTool   JinnangType = "tool"  // 工具锦囊
	JinnangWisdom JinnangType = "wisdom" // 智慧锦囊
)

// Jinnang 锦囊定义
type Jinnang struct {
	ID          string                 `json:"id" yaml:"id"`
	Name        string                 `json:"name" yaml:"name"`
	Type        JinnangType          `json:"type" yaml:"type"`
	Description string                 `json:"description" yaml:"description"`
	Icon        string                 `json:"icon" yaml:"icon"`
	Version     string                 `json:"version" yaml:"version"`
	Tags        []string               `json:"tags" yaml:"tags"`
	Config      map[string]interface{} `json:"config" yaml:"config"`
	CreatedAt   time.Time             `json:"created_at" yaml:"created_at"`
	UpdatedAt   time.Time             `json:"updated_at" yaml:"updated_at"`
}

// JinnangInstance 锦囊实例
type JinnangInstance struct {
	*Jinnang
	Handler JinnangHandler
	State   JinnangState
}

// JinnangState 锦囊状态
type JinnangState int

const (
	JinnangStateInactive JinnangState = iota
	JinnangStateActive
	JinnangStateError
)

// JinnangHandler 锦囊处理器接口
type JinnangHandler interface {
	// Execute 执行锦囊
	Execute(ctx context.Context, input JinnangInput) (*JinnangOutput, error)

	// Validate 验证输入
	Validate(input JinnangInput) error

	// GetSchema 获取输入输出Schema
	GetSchema() (*JinnangSchema, error)
}

// JinnangInput 锦囊输入
type JinnangInput struct {
	Context map[string]interface{} `json:"context" yaml:"context"`
	Params  map[string]interface{} `json:"params" yaml:"params"`
	Data    interface{}            `json:"data" yaml:"data"`
}

// JinnangOutput 锦囊输出
type JinnangOutput struct {
	Success bool                   `json:"success" yaml:"success"`
	Data    interface{}            `json:"data" yaml:"data"`
	Error   string                 `json:"error,omitempty" yaml:"error,omitempty"`
	Meta    map[string]interface{} `json:"meta,omitempty" yaml:"meta,omitempty"`
}

// JinnangSchema 锦囊Schema
type JinnangSchema struct {
	Input  map[string]interface{} `json:"input" yaml:"input"`
	Output map[string]interface{} `json:"output" yaml:"output"`
}

// Vault 锦囊库接口
type Vault interface {
	// Register 注册锦囊
	Register(jinnang *Jinnang, handler JinnangHandler) error

	// Unregister 注销锦囊
	Unregister(id string) error

	// Get 获取锦囊
	Get(id string) (*JinnangInstance, error)

	// List 列出锦囊
	List(filter JinnangFilter) []*Jinnang

	// Execute 执行锦囊
	Execute(ctx context.Context, id string, input JinnangInput) (*JinnangOutput, error)

	// LoadFromDir 从目录加载锦囊
	LoadFromDir(dir string) error
}

// JinnangFilter 锦囊过滤器
type JinnangFilter struct {
	Type JinnangType
	Tags []string
}

// DefaultVault 锦囊库默认实现
type DefaultVault struct {
	mu       sync.RWMutex
	jinnangs map[string]*JinnangInstance
}

// NewVault 创建锦囊库
func NewVault() *DefaultVault {
	return &DefaultVault{
		jinnangs: make(map[string]*JinnangInstance),
	}
}

// Register 注册锦囊
func (v *DefaultVault) Register(jinnang *Jinnang, handler JinnangHandler) error {
	v.mu.Lock()
	defer v.mu.Unlock()

	if jinnang.ID == "" {
		return fmt.Errorf("锦囊ID不能为空")
	}

	now := time.Now()
	jinnang.CreatedAt = now
	jinnang.UpdatedAt = now

	v.jinnangs[jinnang.ID] = &JinnangInstance{
		Jinnang: jinnang,
		Handler: handler,
		State:   JinnangStateActive,
	}

	return nil
}

// Unregister 注销锦囊
func (v *DefaultVault) Unregister(id string) error {
	v.mu.Lock()
	defer v.mu.Unlock()

	if _, exists := v.jinnangs[id]; !exists {
		return fmt.Errorf("锦囊不存在: %s", id)
	}

	delete(v.jinnangs, id)
	return nil
}

// Get 获取锦囊
func (v *DefaultVault) Get(id string) (*JinnangInstance, error) {
	v.mu.RLock()
	defer v.mu.RUnlock()

	jinnang, exists := v.jinnangs[id]
	if !exists {
		return nil, fmt.Errorf("锦囊不存在: %s", id)
	}

	return jinnang, nil
}

// List 列出锦囊
func (v *DefaultVault) List(filter JinnangFilter) []*Jinnang {
	v.mu.RLock()
	defer v.mu.RUnlock()

	var result []*Jinnang
	for _, j := range v.jinnangs {
		if filter.Type != "" && j.Type != filter.Type {
			continue
		}
		if len(filter.Tags) > 0 {
			hasTag := false
			for _, tag := range filter.Tags {
				for _, jTag := range j.Tags {
					if tag == jTag {
						hasTag = true
						break
					}
				}
			}
			if !hasTag {
				continue
			}
		}
		result = append(result, j.Jinnang)
	}

	return result
}

// Execute 执行锦囊
func (v *DefaultVault) Execute(ctx context.Context, id string, input JinnangInput) (*JinnangOutput, error) {
	jinnang, err := v.Get(id)
	if err != nil {
		return nil, err
	}

	// 验证输入
	if err := jinnang.Handler.Validate(input); err != nil {
		return &JinnangOutput{
			Success: false,
			Error:   fmt.Sprintf("输入验证失败: %v", err),
		}, nil
	}

	// 执行
	return jinnang.Handler.Execute(ctx, input)
}

// LoadFromDir 从目录加载锦囊
func (v *DefaultVault) LoadFromDir(dir string) error {
	// TODO: 实现从目录加载锦囊
	return nil
}

// ===== 预置锦囊 =====

// HuogongJinnang 火攻锦囊 - 数据处理
type HuogongJinnang struct{}

func (h *HuogongJinnang) Execute(ctx context.Context, input JinnangInput) (*JinnangOutput, error) {
	// 火攻：快速处理数据
	data, ok := input.Data.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("火攻需要map类型数据")
	}

	// 模拟数据处理
	result := make(map[string]interface{})
	for k, v := range data {
		result[k] = fmt.Sprintf("processed_%v", v)
	}

	return &JinnangOutput{
		Success: true,
		Data:    result,
		Meta:    map[string]interface{}{"method": "huogong"},
	}, nil
}

func (h *HuogongJinnang) Validate(input JinnangInput) error {
	if input.Data == nil {
		return fmt.Errorf("火攻需要数据")
	}
	return nil
}

func (h *HuogongJinnang) GetSchema() (*JinnangSchema, error) {
	return &JinnangSchema{
		Input: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"data": "any",
			},
		},
		Output: map[string]interface{}{
			"type": "object",
		},
	}, nil
}

// ShuiboJinnang 水淹锦囊 - 流式处理
type ShuiboJinnang struct{}

func (s *ShuiboJinnang) Execute(ctx context.Context, input JinnangInput) (*JinnangOutput, error) {
	// 水淹：流式处理数据
	stream, ok := input.Data.([]interface{})
	if !ok {
		return nil, fmt.Errorf("水淹需要数组类型数据")
	}

	results := make([]interface{}, 0, len(stream))
	for _, item := range stream {
		// 模拟流式处理
		results = append(results, map[string]interface{}{
			"original": item,
			"flowed":   true,
		})
	}

	return &JinnangOutput{
		Success: true,
		Data:    results,
		Meta:    map[string]interface{}{"method": "shuibo", "count": len(stream)},
	}, nil
}

func (s *ShuiboJinnang) Validate(input JinnangInput) error {
	return nil
}

func (s *ShuiboJinnang) GetSchema() (*JinnangSchema, error) {
	return &JinnangSchema{
		Input: map[string]interface{}{
			"type": "array",
		},
		Output: map[string]interface{}{
			"type": "array",
		},
	}, nil
}

// KongchengJinnang 空城锦囊 - 智能调度
type KongchengJinnang struct{}

func (k *KongchengJinnang) Execute(ctx context.Context, input JinnangInput) (*JinnangOutput, error) {
	// 空城：智能调度，虚虚实实
	task, ok := input.Params["task"].(string)
	if !ok {
		task = "unknown"
	}

	// 模拟智能决策
	strategy := "保守策略"
	if risk, ok := input.Params["risk_level"].(float64); ok && risk > 0.7 {
		strategy = "激进策略"
	}

	return &JinnangOutput{
		Success: true,
		Data: map[string]interface{}{
			"task":     task,
			"strategy": strategy,
			"note":     "虚者实之，实者虚之",
		},
		Meta: map[string]interface{}{"method": "kongcheng"},
	}, nil
}

func (k *KongchengJinnang) Validate(input JinnangInput) error {
	return nil
}

func (k *KongchengJinnang) GetSchema() (*JinnangSchema, error) {
	return &JinnangSchema{
		Input: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"task":       "string",
				"risk_level": "number",
			},
		},
		Output: map[string]interface{}{
			"type": "object",
		},
	}, nil
}

// ===== Skill 注册辅助函数 =====

// SkillHandler skill处理器接口（兼容旧接口）
type SkillHandler interface {
	Name() string
	Execute(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error)
}

// SkillAdapter Skill适配器
type SkillAdapter struct {
	name    string
	handler SkillHandler
}

func (s *SkillAdapter) Execute(ctx context.Context, input JinnangInput) (*JinnangOutput, error) {
	params := input.Params
	if params == nil {
		params = make(map[string]interface{})
	}
	if input.Context != nil {
		for k, v := range input.Context {
			params[k] = v
		}
	}

	result, err := s.handler.Execute(ctx, params)
	if err != nil {
		return &JinnangOutput{
			Success: false,
			Error:   err.Error(),
		}, nil
	}

	return &JinnangOutput{
		Success: true,
		Data:    result,
	}, nil
}

func (s *SkillAdapter) Validate(input JinnangInput) error {
	return nil
}

func (s *SkillAdapter) GetSchema() (*JinnangSchema, error) {
	return &JinnangSchema{
		Input:  map[string]interface{}{"type": "object"},
		Output: map[string]interface{}{"type": "object"},
	}, nil
}

// RegisterSkill 注册skill（便捷方法）
func (v *DefaultVault) RegisterSkill(name string, handler SkillHandler) error {
	jinnang := &Jinnang{
		ID:   name,
		Name: name,
		Type: JinnangSkill,
	}
	return v.Register(jinnang, &SkillAdapter{name: name, handler: handler})
}
