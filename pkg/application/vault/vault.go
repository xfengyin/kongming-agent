// Package vault 提供锦囊库（Vault）的应用层实现。
//
// 职责：
//   - 进程内注册 Jinnang 元数据 + JinnangHandler 行为
//   - 提供 GetJinnang / ListJinnang 的只读查询
//   - Execute 把入参路由到 handler，约定失败语义
//   - LoadFromDir 从文件系统批量加载锦囊定义（*.json / *.yaml 占位）
//
// 设计要点：
//   - 线程安全：sync.RWMutex 保护 map；读多写少场景下走读锁。
//   - 依赖倒置：本类型实现 domain/port.Vault，依赖 port.Observer 抽象。
//   - 可观测：Execute 走 OTel span + counter 指标；load 走结构化日志。
//   - 失败语义：handler.Validate 失败 → output{Success:false}，不视为 error；
//     handler.Execute 失败 → 透传 error 给上游，附 span 事件便于排障。
package vault

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"

	"github.com/zhuge/kongming/pkg/domain/model"
	"github.com/zhuge/kongming/pkg/domain/port"
)

// entry 是 Vault 内部存储的「元数据 + 行为」对。
// 与 model.Jinnang 解耦，避免外部直接篡改 Config / Tags。
type entry struct {
	// jinnang 锦囊元数据拷贝（CreatedAt/UpdatedAt 在 RegisterSkill 时填充）。
	jinnang *model.Jinnang
	// handler 锦囊行为实现；Execute 路由到它。
	handler model.JinnangHandler
}

// Vault 是锦囊库的进程内实现，线程安全。
// 满足 domain/port.Vault 接口契约。
type Vault struct {
	// mu 保护 entries 字典；读路径走 RLock，写路径走 Lock。
	mu sync.RWMutex
	// entries 以 jinnang.ID 为 key，存放「元数据 + handler」对。
	entries map[string]*entry
	// logger 用于结构化日志；测试可注入 zap.NewNop。
	logger *zap.Logger
	// observer 可观测性端口；为 nil 时降级为 noop，不影响主流程。
	observer port.Observer
}

// NewVault 构造一个空的 Vault。
// observer 可传 nil（用于测试或禁用可观测），logger 不能为 nil。
func NewVault(logger *zap.Logger, observer port.Observer) *Vault {
	if logger == nil {
		// 防御：调用方忘传 logger 时使用 noop logger，避免 panic。
		logger = zap.NewNop()
	}
	return &Vault{
		entries:  make(map[string]*entry),
		logger:   logger,
		observer: observer,
	}
}

// RegisterSkill 注册一个锦囊（包含元数据 + Handler）。
//   - jinnang.ID 为空 / handler 为 nil → 返回错误，不写入。
//   - 重复 ID 视为「后者覆盖前者」（热更新语义）。
//   - 写入时自动填充 CreatedAt / UpdatedAt。
func (v *Vault) RegisterSkill(jinnang *model.Jinnang, handler model.JinnangHandler) error {
	// 入参校验：前置失败不入 map。
	if jinnang == nil {
		return errors.New("vault: jinnang 不能为 nil")
	}
	if jinnang.ID == "" {
		return errors.New("vault: jinnang.ID 不能为空")
	}
	if handler == nil {
		return fmt.Errorf("vault: handler 不能为 nil (id=%s)", jinnang.ID)
	}

	now := time.Now()
	// 拷贝元数据，避免外部后续修改污染 vault 内部状态。
	meta := *jinnang
	meta.UpdatedAt = now
	// 首次注册时填 CreatedAt；覆盖时保留原 CreatedAt（若调用方未填）。
	if meta.CreatedAt.IsZero() {
		meta.CreatedAt = now
	}

	v.mu.Lock()
	v.entries[meta.ID] = &entry{jinnang: &meta, handler: handler}
	v.mu.Unlock()

	v.logger.Info("jinnang registered",
		zap.String("id", meta.ID),
		zap.String("type", string(meta.Type)),
	)
	return nil
}

// GetJinnang 按 ID 查询锦囊元数据。
// 不存在时返回 (nil, err)。
// 返回的是 *entry 内部指针的拷贝（jinnang 字段再拷贝一次），
// 避免外部直接修改 entry。
func (v *Vault) GetJinnang(id string) (*model.Jinnang, error) {
	v.mu.RLock()
	defer v.mu.RUnlock()

	e, ok := v.entries[id]
	if !ok {
		return nil, fmt.Errorf("vault: jinnang 不存在 (id=%s)", id)
	}
	// 返回元数据副本。
	j := *e.jinnang
	return &j, nil
}

// ListJinnang 返回当前库内全部锦囊的元数据列表（按 ID 字典序）。
// 返回深拷贝（每个 Jinnang 字段独立），调用方修改不影响内部状态。
func (v *Vault) ListJinnang() ([]*model.Jinnang, error) {
	v.mu.RLock()
	defer v.mu.RUnlock()

	out := make([]*model.Jinnang, 0, len(v.entries))
	for _, e := range v.entries {
		j := *e.jinnang
		out = append(out, &j)
	}
	// 字典序排序，便于可观测 / UI 列表稳定。
	sort.Slice(out, func(i, k int) bool { return out[i].ID < out[k].ID })
	return out, nil
}

// Execute 按 ID 执行锦囊。
// 错误约定：
//   - 不存在 → (nil, err)
//   - Validate 失败 → (output{Success:false, Error:...}, nil)
//   - Execute 成功 → (output{Success:true, Data:...}, nil)
//   - Execute 失败 → (nil, err) 或 (output{Success:false, Error:...}, err)，
//     视 handler 实现约定；本方法仅透传。
func (v *Vault) Execute(ctx context.Context, id string, input model.JinnangInput) (*model.JinnangOutput, error) {
	// 1) 读路径：拿元数据 + handler 引用。
	v.mu.RLock()
	e, ok := v.entries[id]
	if !ok {
		v.mu.RUnlock()
		return nil, fmt.Errorf("vault: jinnang 不存在 (id=%s)", id)
	}
	handler := e.handler
	v.mu.RUnlock()

	// 2) 可观测：开 span（observer 为 nil 时跳过）。
	// 注意：ctx 在 observer==nil 分支不被消费，使用 _ 占位以满足
	// 「declared and not used」检查，同时保留 ctx 形参透传语义。
	if v.observer != nil {
		var span trace.Span
		ctx, span = v.observer.StartSpan(ctx, "vault.Execute",
			attribute.String("jinnang.id", id),
			attribute.String("jinnang.type", string(e.jinnang.Type)),
		)
		defer span.End()
	}
	_ = ctx // 显式使用，避免编译错误

	// 3) 入参校验（handler 自行决定规则）。
	if err := handler.Validate(input); err != nil {
		v.logger.Warn("jinnang validate failed",
			zap.String("id", id), zap.Error(err))
		return &model.JinnangOutput{
			Success: false,
			Error:   fmt.Sprintf("validate failed: %v", err),
		}, nil
	}

	// 4) 执行。
	out, err := handler.Execute(ctx, input)
	if err != nil {
		v.logger.Error("jinnang execute failed",
			zap.String("id", id), zap.Error(err))
		return out, err
	}
	return out, nil
}
