package service

import (
	"context"

	pb "github.com/zhuge/kongming/api/proto/kongming/v1"
	domerrs "github.com/zhuge/kongming/pkg/domain/errors"
	"github.com/zhuge/kongming/pkg/domain/model"
)

// ListJinnang 实现 KongmingServer.ListJinnang。
//
// 返回当前 Vault 中全部锦囊元数据；库为空时返回空列表（不报错）。
func (s *Service) ListJinnang(ctx context.Context, _ *pb.ListJinnangRequest) (*pb.ListJinnangResponse, error) {
	if s.v == nil {
		return nil, toGRPCStatus(domerrs.New(domerrs.INTERNAL, "vault is not configured"))
	}
	jinnangs, err := s.v.ListJinnang()
	if err != nil {
		return nil, toGRPCStatus(err)
	}
	resp := &pb.ListJinnangResponse{Jinnangs: make([]*pb.Jinnang, 0, len(jinnangs))}
	for _, j := range jinnangs {
		resp.Jinnangs = append(resp.Jinnangs, jinnangToProto(j))
	}
	return resp, nil
}

// ExecuteJinnang 实现 KongmingServer.ExecuteJinnang。
//
// 入参解析：
//  1. id 必填，否则 INVALID_ARGUMENT；
//  2. params（string→string）转为 Params（string→any），便于 handler 解释；
//  3. data（bytes）原样塞入 Data（handler 自行反序列化）。
func (s *Service) ExecuteJinnang(ctx context.Context, req *pb.ExecuteJinnangRequest) (*pb.ExecuteJinnangResponse, error) {
	if s.v == nil {
		return nil, toGRPCStatus(domerrs.New(domerrs.INTERNAL, "vault is not configured"))
	}
	if req == nil {
		return nil, toGRPCStatus(domerrs.New(domerrs.INVALID_ARGUMENT, "req is nil").WithField("rpc", "ExecuteJinnang"))
	}
	if req.GetId() == "" {
		return nil, toGRPCStatus(
			domerrs.New(domerrs.INVALID_ARGUMENT, "id is required").WithField("rpc", "ExecuteJinnang"),
		)
	}

	params := make(map[string]any, len(req.GetParams()))
	for k, v := range req.GetParams() {
		params[k] = v
	}
	input := model.JinnangInput{
		Params: params,
		Data:   req.GetData(),
	}

	output, err := s.v.Execute(ctx, req.GetId(), input)
	if err != nil {
		return nil, toGRPCStatus(err)
	}
	// output == nil 的边界情况：handler 协议错误，但 err 也为 nil。
	// 兜底返回 Success=false，避免 NPE。
	if output == nil {
		return &pb.ExecuteJinnangResponse{Success: false, Error: "empty output"}, nil
	}

	// Data 字段：handler 可放任意类型；优先按 []byte 取，否则 JSON 序列化。
	var dataBytes []byte
	switch v := output.Data.(type) {
	case nil:
		// pass
	case []byte:
		dataBytes = v
	default:
		// 通用兜底：JSON 序列化便于跨语言调用。
		b, _ := jsonMarshal(v)
		dataBytes = b
	}
	return &pb.ExecuteJinnangResponse{
		Success: output.Success,
		Data:    dataBytes,
		Error:   output.Error,
	}, nil
}

// jinnangToProto 把领域 Jinnang 转为 pb.Jinnang。
func jinnangToProto(j *model.Jinnang) *pb.Jinnang {
	if j == nil {
		return nil
	}
	return &pb.Jinnang{
		Id:      j.ID,
		Name:    j.Name,
		Type:    int32(jinnangTypeToInt(j.Type)),
		Version: j.Version,
	}
}

// jinnangTypeToInt 把领域锦囊类型映射到 proto int（与 proto 定义保持稳定）。
//
// proto schema 中没有显式 enum，故此处约定：
//
//	skill=1, tool=2, wisdom=3
//
// 未知值降级为 0（proto3 默认值），调用方可据此判定 "未知类型"。
func jinnangTypeToInt(t model.JinnangType) int {
	switch t {
	case model.JinnangSkill:
		return 1
	case model.JinnangTool:
		return 2
	case model.JinnangWisdom:
		return 3
	default:
		return 0
	}
}
