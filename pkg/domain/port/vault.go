// Package port 定义领域层对外暴露的「端口」（接口）。
//
// 本文件聚焦于「锦囊库 Vault」端口：抽象锦囊（Jinnang）的注册、查询、调用与目录加载。
// application 层通过依赖倒置注入此接口，infra/builtin 与 plugin loader 负责实现具体加载。
//
// 设计原则（六边形架构）：
//  1. 接口最小化：5 个方法覆盖「注册 / 查询 / 列表 / 执行 / 加载」完整用例域。
//  2. ctx 透传：Execute / LoadFromDir 接收 context.Context，便于超时/取消/链路追踪。
//  3. 错误语义：返回 error 而非 bool，统一使用 pkg/domain/errors 包。
//  4. 数据 + 行为分离：Jinnang 仅含元数据，行为由 JinnangHandler 注入；
//     符合开闭原则，新增锦囊类型不修改本接口。
package port

import (
	"context"

	"github.com/zhuge/kongming/pkg/domain/model"
)

// Vault 是锦囊库的统一入口端口。
//
// 实现：pkg/application/vault.Vault。
// 典型用例：Commander 在执行订单时按需调用 Vault.Execute(ctx, jinnangID, input)；
// 启动期通过 Vault.LoadFromDir(ctx, dir) 把 *.json/*.yaml 锦囊清单自动注册。
type Vault interface {
	// RegisterSkill 注册一个锦囊（包含元数据 + Handler）。
	// 重复 ID 视为「后者覆盖前者」（热更新语义）。
	// 入参校验：jinnang.ID 与 handler 均不能为空。
	// 返回 err 表示校验失败；注册成功返回 nil。
	RegisterSkill(jinnang *model.Jinnang, handler model.JinnangHandler) error

	// GetJinnang 按 ID 查询锦囊元数据（不返回 Handler，避免误调用）。
	// 不存在时返回 (nil, err)。
	GetJinnang(id string) (*model.Jinnang, error)

	// ListJinnang 返回当前库内全部锦囊的元数据列表（按 ID 字典序，便于可观测）。
	// 返回拷贝，避免外部修改影响内部状态。
	ListJinnang() ([]*model.Jinnang, error)

	// Execute 按 ID 执行锦囊。
	//   - 不存在 → (nil, err)
	//   - handler.Validate 失败 → (output{Success:false, Error:...}, nil)，
	//     不视作 error 透传给上游，而是约定在 output.Error 中描述。
	//   - handler.Execute 成功 → (output{Success:true, Data:...}, nil)
	//   - handler.Execute 失败 → (nil, err) 或 (output{Success:false, Error:...}, err)，
	//     视具体实现约定。
	Execute(ctx context.Context, id string, input model.JinnangInput) (*model.JinnangOutput, error)

	// LoadFromDir 从指定目录加载锦囊定义文件（*.json/*.yaml）并批量注册。
	// 已存在的 ID 会被覆盖（热更新语义）。
	// 不存在的目录 / 文件应返回 error（不静默忽略）。
	// ctx 用于链路追踪；扫描行为不阻塞，但解析/注册同步执行。
	LoadFromDir(ctx context.Context, dir string) error
}
