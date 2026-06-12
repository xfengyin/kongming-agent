package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	domerrs "github.com/zhuge/kongming/pkg/domain/errors"
	"github.com/zhuge/kongming/pkg/domain/model"
)

// TestPlanStrategy_OK 验证 PlanStrategy 正常返回 200。
func TestPlanStrategy_OK(t *testing.T) {
	mock := &mockCommander{
		planStrategyFn: func(_ context.Context, o *model.Order) (*model.Strategy, error) {
			assert.Equal(t, "战略名", o.Name)
			return &model.Strategy{
				Type:      "offensive",
				BaguaMode: model.Tiangai,
				Tactics:   []model.Tactic{{Order: 1, Name: "t1"}},
			}, nil
		},
	}
	h := newTestHandler(mock)
	r := newTestEngine("POST", "/api/v1/strategies", h.PlanStrategy)

	body := `{"name":"战略名","priority":1,"objectives":["o1"]}`
	req := httptest.NewRequest("POST", "/api/v1/strategies", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "offensive", resp["Type"])
	assert.Equal(t, "tiangai", resp["BaguaMode"])
}

// TestPlanStrategy_StrategyFailed 验证 Strategy 失败映射 500 + STRATEGY_FAILED。
func TestPlanStrategy_StrategyFailed(t *testing.T) {
	mock := &mockCommander{
		planStrategyFn: func(_ context.Context, _ *model.Order) (*model.Strategy, error) {
			return nil, domerrs.New(domerrs.STRATEGY_FAILED, "planner exploded")
		},
	}
	h := newTestHandler(mock)
	r := newTestEngine("POST", "/api/v1/strategies", h.PlanStrategy)

	body := `{"name":"x"}`
	req := httptest.NewRequest("POST", "/api/v1/strategies", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusInternalServerError, w.Code)
	var resp dtoErrorResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "STRATEGY_FAILED", string(resp.Code))
}

// TestPlanStrategy_BadRequest 验证 name 缺失返回 400。
func TestPlanStrategy_BadRequest(t *testing.T) {
	h := newTestHandler(&mockCommander{})
	r := newTestEngine("POST", "/api/v1/strategies", h.PlanStrategy)

	req := httptest.NewRequest("POST", "/api/v1/strategies", bytes.NewBufferString(`{"description":"d"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code)
}
