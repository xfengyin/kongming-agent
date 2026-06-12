package service

import (
	"context"
	"encoding/json"

	pb "github.com/zhuge/kongming/api/proto/kongming/v1"
	domerrs "github.com/zhuge/kongming/pkg/domain/errors"
	"github.com/zhuge/kongming/pkg/domain/model"
)

// RunWorkflow 实现 KongmingServer.RunWorkflow。
//
// 行为：
//  1. workflow_id 必填；
//  2. inputs（string→string）转为 map[string]any 注入 ExecutionContext.Variables；
//  3. 返回 ExecutionContext.NodeStates 的 string→string 投影（status 字符串）。
func (s *Service) RunWorkflow(ctx context.Context, req *pb.RunWorkflowRequest) (*pb.RunWorkflowResponse, error) {
	if s.e == nil {
		return nil, toGRPCStatus(domerrs.New(domerrs.INTERNAL, "engine is not configured"))
	}
	if req == nil {
		return nil, toGRPCStatus(domerrs.New(domerrs.INVALID_ARGUMENT, "req is nil").WithField("rpc", "RunWorkflow"))
	}
	if req.GetWorkflowId() == "" {
		return nil, toGRPCStatus(
			domerrs.New(domerrs.INVALID_ARGUMENT, "workflow_id is required").WithField("rpc", "RunWorkflow"),
		)
	}

	inputs := make(map[string]any, len(req.GetInputs()))
	for k, v := range req.GetInputs() {
		inputs[k] = v
	}

	ec, err := s.e.Execute(ctx, req.GetWorkflowId(), inputs)
	if err != nil {
		return nil, toGRPCStatus(err)
	}
	if ec == nil {
		return &pb.RunWorkflowResponse{Success: false, Error: "empty execution context"}, nil
	}

	// NodeStates 转 string→string 投影：key=node_id, value=state.Status。
	// NodeState 是值类型（非指针），通过 map 索引直接取零值。
	nodeStates := make(map[string]string, len(ec.NodeStates))
	for id, st := range ec.NodeStates {
		nodeStates[id] = string(st.Status)
	}
	// 整体成功判定：任一节点非 OK 即视为失败。
	success := true
	for _, st := range ec.NodeStates {
		if st.Status != model.NodeStatusOK {
			success = false
			break
		}
	}
	return &pb.RunWorkflowResponse{
		Success:    success,
		NodeStates: nodeStates,
	}, nil
}

// jsonMarshal 是 encoding/json 序列化的小包装，便于单测断言。
func jsonMarshal(v any) ([]byte, error) {
	return json.Marshal(v)
}
