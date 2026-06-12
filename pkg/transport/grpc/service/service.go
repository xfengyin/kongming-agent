// Package service 实现 Kongming gRPC Service 的业务方法。
//
// 设计原则：
//  1. 嵌入 pb.UnimplementedKongmingServer：保证 forward-compat，
//     新增 RPC 时旧实现不会因接口不匹配而编译失败；
//  2. 依赖倒置：所有业务方法只依赖 port.* 接口（Commander/GeneralPool/Vault/Engine），
//     不直接 new 具体实现；测试用 mock 注入；
//  3. 错误统一转换：业务错误一律走 toGRPCStatus 映射，避免散落各处的
//     status.Error(codes.Xxx, ...) 调用；
//  4. 单一职责：每个文件聚焦一类 RPC（order/general/vault/workflow），
//     便于 review 与按需扩展。
package service

import (
	pb "github.com/zhuge/kongming/api/proto/kongming/v1"
	domerrs "github.com/zhuge/kongming/pkg/domain/errors"
	"github.com/zhuge/kongming/pkg/domain/port"
)

// Service 是 Kongming gRPC service 的实现。
//
// 嵌入 UnimplementedKongmingServer 防止接口升级时遗漏方法导致编译失败。
// 五个 port 依赖通过 New 注入，便于测试替换。
type Service struct {
	pb.UnimplementedKongmingServer
	// c 军师用例端口（派单 / 查询订单 / 审核战报）
	c port.Commander
	// d 调度器端口（异步派发 Order）
	d port.Dispatcher
	// e 工作流引擎端口（执行八卦阵 / 工作流）
	e port.Engine
	// p 将领池端口（将领 CRUD / 派单 / 执行）
	p port.GeneralPool
	// v 锦囊库端口（锦囊注册 / 查询 / 执行）
	v port.Vault
}

// New 构造一个 Service。
//
// 入参要求：
//  1. c/v/p 必填（对应 RPC 必须能用）；
//  2. d/e 当前阶段仅保留为 port 依赖（未来 RunWorkflow 走 Engine），
//     但允许为 nil 以便单测时只注入部分依赖。
func New(c port.Commander, d port.Dispatcher, e port.Engine, p port.GeneralPool, v port.Vault) *Service {
	return &Service{c: c, d: d, e: e, p: p, v: v}
}

// toGRPCStatus 把领域错误转为 gRPC status。
//
// 行为：
//  1. err == nil → (nil, nil)，让 caller 透传成功；
//  2. err 是 *domerrs.Error → 使用其 Code.GRPCCode() 映射；消息保留 err.Error()；
//  3. 其他 err → 兜底为 codes.Internal（避免泄漏内部细节）。
//
// 返回值总是兼容 grpc status，可直接由 gRPC 框架序列化。
func toGRPCStatus(err error) error {
	if err == nil {
		return nil
	}
	if de, ok := err.(*domerrs.Error); ok {
		// 计划原文 e.Code.GRPCCode() —— e.Code 是 domerrs.Code 类型常量，
		// 由于 type Code string 是值类型，可直接调用 *Code 方法。
		// 为类型安全保留一次显式转换，避免未来 fields 字段为 interface 时混淆。
		return grpcStatus(domerrs.Code(de.Code).GRPCCode(), de.Error())
	}
	// 兜底：未知错误一律 Internal。
	return grpcStatusInternal(err.Error())
}
