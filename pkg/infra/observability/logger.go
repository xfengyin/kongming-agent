// Package observability 提供可观测性三大支柱的轻量实现：
//   - 结构化日志（zap）
//   - 链路追踪（OTel + OTLP HTTP exporter）
//   - 指标（Prometheus）
//
// 以及贯穿全链路的 traceId 透传工具。
package observability

import (
	"fmt"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/zhuge/kongming/pkg/infra/config"
)

// NewLogger 根据配置创建 zap 结构化日志器。
// encoding 为空时默认 json；level 解析失败时回退到 info。
func NewLogger(cfg config.LogConfig) (*zap.Logger, error) {
	enc := cfg.Encoding
	if enc == "" {
		enc = "json"
	}

	zcfg := zap.Config{
		Level:       zap.NewAtomicLevelAt(ParseLevel(cfg.Level)),
		Development: false,
		Encoding:    enc,
		EncoderConfig: zapcore.EncoderConfig{
			TimeKey:        "ts",
			LevelKey:       "level",
			NameKey:        "logger",
			CallerKey:      "caller",
			FunctionKey:    zapcore.OmitKey,
			MessageKey:     "msg",
			StacktraceKey:  "stacktrace",
			LineEnding:     zapcore.DefaultLineEnding,
			EncodeLevel:    zapcore.LowercaseLevelEncoder,
			EncodeTime:     zapcore.ISO8601TimeEncoder,
			EncodeDuration: zapcore.SecondsDurationEncoder,
			EncodeCaller:   zapcore.ShortCallerEncoder,
		},
		OutputPaths:      []string{"stdout"},
		ErrorOutputPaths: []string{"stderr"},
	}

	logger, err := zcfg.Build()
	if err != nil {
		return nil, fmt.Errorf("build logger: %w", err)
	}
	return logger, nil
}

// ParseLevel 将字符串解析为 zap 日志级别，未知值回退到 InfoLevel。
// 同时校验合法性——若 encoding 配置为无效值（虽然 NewLogger 不强制）也能兜底。
func ParseLevel(level string) zapcore.Level {
	var l zapcore.Level
	if err := l.UnmarshalText([]byte(level)); err != nil {
		return zapcore.InfoLevel
	}
	return l
}
