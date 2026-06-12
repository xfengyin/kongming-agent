package service

import (
	"context"

	pb "github.com/zhuge/kongming/api/proto/kongming/v1"
	domerrs "github.com/zhuge/kongming/pkg/domain/errors"
	"github.com/zhuge/kongming/pkg/domain/model"
)

// ListGenerals 实现 KongmingServer.ListGenerals。
//
// 返回池中全量将领快照；池为空时返回空列表（不报错）。
// 内部调用 port.GeneralPool.List，与 Commander 的将领选择解耦。
func (s *Service) ListGenerals(ctx context.Context, _ *pb.ListGeneralsRequest) (*pb.ListGeneralsResponse, error) {
	if s.p == nil {
		return nil, toGRPCStatus(domerrs.New(domerrs.INTERNAL, "general pool is not configured"))
	}
	generals, err := s.p.List(ctx)
	if err != nil {
		return nil, toGRPCStatus(err)
	}
	resp := &pb.ListGeneralsResponse{Generals: make([]*pb.General, 0, len(generals))}
	for _, g := range generals {
		resp.Generals = append(resp.Generals, generalToProto(g))
	}
	return resp, nil
}

// generalToProto 把领域 General 转为 pb.General。
func generalToProto(g *model.General) *pb.General {
	if g == nil {
		return nil
	}
	return &pb.General{
		Id:     string(g.ID),
		Name:   g.Name,
		Title:  g.Title,
		Skills: append([]string(nil), g.Skills...),
		State:  int32(g.State),
	}
}
