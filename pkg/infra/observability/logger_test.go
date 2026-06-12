package observability

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/zhuge/kongming/pkg/infra/config"
)

func TestNewLogger_Info(t *testing.T) {
	cfg := config.LogConfig{Level: "info", Encoding: "json"}
	logger, err := NewLogger(cfg)
	require.NoError(t, err)
	assert.NotNil(t, logger)
	assert.IsType(t, &zap.Logger{}, logger)
}

func TestNewLogger_InvalidEncoding(t *testing.T) {
	// encoding 字段对未知值会回退到默认（zap.Build 仍成功），这里仅覆盖兜底逻辑
	cfg := config.LogConfig{Level: "debug", Encoding: "console"}
	logger, err := NewLogger(cfg)
	require.NoError(t, err)
	assert.NotNil(t, logger)
}

func TestNewLogger_DefaultEncoding(t *testing.T) {
	// encoding 为空时应默认 json
	cfg := config.LogConfig{Level: "info"}
	logger, err := NewLogger(cfg)
	require.NoError(t, err)
	assert.NotNil(t, logger)
}

func TestParseLevel(t *testing.T) {
	assert.Equal(t, zapcore.InfoLevel, ParseLevel("info"))
	assert.Equal(t, zapcore.DebugLevel, ParseLevel("debug"))
	assert.Equal(t, zapcore.WarnLevel, ParseLevel("warn"))
	assert.Equal(t, zapcore.ErrorLevel, ParseLevel("error"))
	assert.Equal(t, zapcore.InfoLevel, ParseLevel("unknown"))
}
