package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/google/uuid"
	pb "github.com/zhuge/kongming/api/proto/kongming/v1"
	domerrs "github.com/zhuge/kongming/pkg/domain/errors"
	"github.com/zhuge/kongming/pkg/domain/model"
)

// Dispatch 实现 KongmingServer.Dispatch。
//
// 业务流：
//  1. 校验入参（Name 必填，Priority 在合理范围）；
//  2. 构造领域 Order（生成 UUID、填充 Strategy.Objectives / Context）；
//  3. 调用 Commander.Dispatch 获取 BattleReport；
//  4. 序列化 BattleReport 为 JSON 填入 report_json（便于 Reviewer/CLI 直接展示）。
func (s *Service) Dispatch(ctx context.Context, req *pb.DispatchRequest) (*pb.DispatchResponse, error) {
	if req == nil {
		return nil, toGRPCStatus(domerrs.New(domerrs.INVALID_ARGUMENT, "req is nil").WithField("rpc", "Dispatch"))
	}
	if req.GetName() == "" {
		return nil, toGRPCStatus(
			domerrs.New(domerrs.INVALID_ARGUMENT, "name is required").WithField("rpc", "Dispatch"),
		)
	}
	priority := model.Priority(req.GetPriority())
	if priority < model.PriorityLow || priority > model.PriorityUrgent {
		return nil, toGRPCStatus(
			domerrs.New(domerrs.INVALID_ARGUMENT, "priority must be 1..4").
				WithField("rpc", "Dispatch").
				WithField("priority", int(priority)),
		)
	}

	// context 字段从 string 转为 any，便于 Commander 内部按 stringer 解释。
	ctxMap := make(map[string]any, len(req.GetContext()))
	for k, v := range req.GetContext() {
		ctxMap[k] = v
	}

	order := &model.Order{
		ID:          model.OrderID(uuid.NewString()),
		Name:        req.GetName(),
		Description: req.GetDescription(),
		State:       model.StatePending,
		Priority:    priority,
		Strategy: model.Strategy{
			Objectives: append([]string(nil), req.GetObjectives()...),
		},
		Context:   ctxMap,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}

	report, err := s.c.Dispatch(ctx, order)
	if err != nil {
		return nil, toGRPCStatus(err)
	}

	// 序列化 BattleReport 为 JSON 字符串，便于客户端直接解析。
	// 序列化失败时不影响主流程：返回空字符串而非阻断。
	reportJSON, _ := json.Marshal(report)

	return &pb.DispatchResponse{
		OrderId:    string(order.ID),
		Success:    report.Success,
		Message:    fmt.Sprintf("dispatched %d general(s)", len(report.Generals)),
		ReportJson: string(reportJSON),
	}, nil
}

// GetOrder 实现 KongmingServer.GetOrder。
//
// 不存在时返回 domerrs.NOT_FOUND（→ gRPC codes.NotFound），由 toGRPCStatus 转换。
func (s *Service) GetOrder(ctx context.Context, req *pb.GetOrderRequest) (*pb.Order, error) {
	if req == nil {
		return nil, toGRPCStatus(domerrs.New(domerrs.INVALID_ARGUMENT, "req is nil").WithField("rpc", "GetOrder"))
	}
	if req.GetId() == "" {
		return nil, toGRPCStatus(
			domerrs.New(domerrs.INVALID_ARGUMENT, "id is required").WithField("rpc", "GetOrder"),
		)
	}
	order, err := s.c.GetOrder(ctx, model.OrderID(req.GetId()))
	if err != nil {
		return nil, toGRPCStatus(err)
	}
	return orderToProto(order), nil
}

// ListOrders 实现 KongmingServer.ListOrders。
//
// state_filter 语义：0 表示不过滤（与 model.StateNone 一致）。
func (s *Service) ListOrders(ctx context.Context, req *pb.ListOrdersRequest) (*pb.ListOrdersResponse, error) {
	if req == nil {
		return nil, toGRPCStatus(domerrs.New(domerrs.INVALID_ARGUMENT, "req is nil").WithField("rpc", "ListOrders"))
	}
	state := model.State(req.GetStateFilter())
	orders, err := s.c.ListOrders(ctx, state)
	if err != nil {
		return nil, toGRPCStatus(err)
	}
	resp := &pb.ListOrdersResponse{Orders: make([]*pb.Order, 0, len(orders))}
	for _, o := range orders {
		resp.Orders = append(resp.Orders, orderToProto(o))
	}
	return resp, nil
}

// orderToProto 把领域 Order 转为 pb.Order。
//
// nil → nil；CreatedAt 零值时 timestamp 字段为 nil（proto3 行为）。
func orderToProto(o *model.Order) *pb.Order {
	if o == nil {
		return nil
	}
	out := &pb.Order{
		Id:       string(o.ID),
		Name:     o.Name,
		State:    int32(o.State),
		Priority: int32(o.Priority),
	}
	if !o.CreatedAt.IsZero() {
		out.CreatedAt = timestamppb.New(o.CreatedAt)
	}
	return out
}
