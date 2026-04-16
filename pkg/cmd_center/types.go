// 军师府 - 核心决策与任务调度
// 运筹帷幄之中，决胜千里之外

package cmd_center

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// 常量定义
const (
	DefaultTimeout = 30 * time.Second // 默认超时时间
)

// TaskState 任务状态
type TaskState int

const (
	StatePending TaskState = iota // 待处理
	StatePlanning                // 谋划中
	StateExecuting               // 执行中
	StateReviewing               // 审核中
	StateCompleted               // 已完成
	StateFailed                  // 失败
)

func (s TaskState) String() string {
	switch s {
	case StatePending:
		return "待处理"
	case StatePlanning:
		return "谋划中"
	case StateExecuting:
		return "执行中"
	case StateReviewing:
		return "审核中"
	case StateCompleted:
		return "已完成"
	case StateFailed:
		return "失败"
	default:
		return "未知"
	}
}

// TaskPriority 任务优先级
type TaskPriority int

const (
	PriorityLow     TaskPriority = iota // 低优先级
	PriorityNormal                      // 普通优先级
	PriorityHigh                        // 高优先级
	PriorityUrgent                      // 紧急军情
)

// MilitaryOrder 军令（任务定义）
type MilitaryOrder struct {
	ID          string                 `json:"id" yaml:"id"`
	Name        string                 `json:"name" yaml:"name"`
	Description string                 `json:"description" yaml:"description"`
	State       TaskState              `json:"state" yaml:"state"`
	Priority    TaskPriority           `json:"priority" yaml:"priority"`
	Strategy    Strategy               `json:"strategy" yaml:"strategy"`
	Context     map[string]interface{} `json:"context" yaml:"context"`
	CreatedAt   time.Time             `json:"created_at" yaml:"created_at"`
	UpdatedAt   time.Time             `json:"updated_at" yaml:"updated_at"`
	Deadline    *time.Time            `json:"deadline,omitempty" yaml:"deadline,omitempty"`
	ParentID    string                 `json:"parent_id,omitempty" yaml:"parent_id,omitempty"`
	GeneralIDs  []string               `json:"general_ids,omitempty" yaml:"general_ids,omitempty"`
}

// NewMilitaryOrder 创建新军令
func NewMilitaryOrder(name, description string, priority TaskPriority) *MilitaryOrder {
	now := time.Now()
	return &MilitaryOrder{
		ID:          uuid.New().String(),
		Name:        name,
		Description: description,
		State:       StatePending,
		Priority:    priority,
		Context:     make(map[string]interface{}),
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}

// Strategy 战略方案
type Strategy struct {
	Type       string   `json:"type" yaml:"type"` // 战略类型
	Objectives []string `json:"objectives" yaml:"objectives"` // 战略目标
	Tactics    []Tactic `json:"tactics" yaml:"tactics"` // 战术步骤
	BaguaMode  string   `json:"bagua_mode" yaml:"bagua_mode"` // 八卦阵模式
	Generals   []string `json:"generals" yaml:"generals"` // 派遣将领
	JinnangIDs []string `json:"jinnang_ids" yaml:"jinnang_ids"` // 使用锦囊
}

// Tactic 战术步骤
type Tactic struct {
	Order       int                    `json:"order" yaml:"order"`
	Name        string                 `json:"name" yaml:"name"`
	Description string                 `json:"description" yaml:"description"`
	Action      string                 `json:"action" yaml:"action"`
	Params      map[string]interface{} `json:"params" yaml:"params"`
	DependsOn   []int                  `json:"depends_on" yaml:"depends_on"`
}

// BattleReport 战报
type BattleReport struct {
	OrderID     string                 `json:"order_id" yaml:"order_id"`
	Success     bool                   `json:"success" yaml:"success"`
	Message     string                 `json:"message" yaml:"message"`
	Data        map[string]interface{} `json:"data" yaml:"data"`
	StartedAt   time.Time             `json:"started_at" yaml:"started_at"`
	CompletedAt time.Time             `json:"completed_at" yaml:"completed_at"`
	Generals    []GeneralReport        `json:"generals" yaml:"generals"`
}

// GeneralReport 将领战报
type GeneralReport struct {
	GeneralID   string                 `json:"general_id" yaml:"general_id"`
	GeneralName string                 `json:"general_name" yaml:"general_name"`
	Success     bool                   `json:"success" yaml:"success"`
	Message     string                 `json:"message" yaml:"message"`
	Data        map[string]interface{} `json:"data" yaml:"data"`
}

// Commander 军师接口
type Commander interface {
	// Dispatch 颁布军令
	Dispatch(ctx context.Context, order *MilitaryOrder) (*BattleReport, error)

	// PlanStrategy 制定战略
	PlanStrategy(ctx context.Context, order *MilitaryOrder) (*Strategy, error)

	// Review 审核战报
	Review(ctx context.Context, report *BattleReport) error

	// GetOrder 查询军令状态
	GetOrder(orderID string) (*MilitaryOrder, error)

	// ListOrders 列出军令
	ListOrders(state TaskState) []*MilitaryOrder
}

// Event 事件
type Event struct {
	Type      string                 `json:"type" yaml:"type"`
	OrderID   string                 `json:"order_id" yaml:"order_id"`
	Timestamp time.Time             `json:"timestamp" yaml:"timestamp"`
	Data      map[string]interface{} `json:"data" yaml:"data"`
}

// EventHandler 事件处理器
type EventHandler func(ctx context.Context, event Event) error
