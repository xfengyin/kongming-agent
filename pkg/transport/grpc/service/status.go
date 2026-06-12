package service

import (
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// grpcStatus 是 status.Error 的薄包装，便于单测 mock 注入。
//
// 设计动机：直接调用 status.Error 会让 toGRPCStatus 强依赖 google.golang.org/grpc，
// 不利于单测构造错误。本函数即"语义保留 + 显式调用"，二者解耦后
// 任何 toGRPCStatus 的失败分支都能通过 status.Code 断言精准定位。
func grpcStatus(c codes.Code, msg string) error {
	return status.Error(c, msg)
}

// grpcStatusInternal 是 Internal 兜底的便捷封装，减少 toGRPCStatus 中的重复样板。
func grpcStatusInternal(msg string) error {
	return status.Error(codes.Internal, msg)
}
