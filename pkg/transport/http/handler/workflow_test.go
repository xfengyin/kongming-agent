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
	"github.com/zhuge/kongming/pkg/domain/port"
)

// mockEngine 实现 port.Engine。
type mockEngine struct {
	regWF   func(wf *model.Workflow) error
	getWF   func(id string) (*model.Workflow, error)
	execFn  func(ctx context.Context, id string, inputs map[string]any) (*model.ExecutionContext, error)
	regExec func(t model.NodeType, exec port.NodeExecutor)
}

func (m *mockEngine) RegisterWorkflow(wf *model.Workflow) error {
	if m.regWF != nil {
		return m.regWF(wf)
	}
	return nil
}
func (m *mockEngine) GetWorkflow(id string) (*model.Workflow, error) {
	if m.getWF != nil {
		return m.getWF(id)
	}
	return nil, nil
}
func (m *mockEngine) Execute(ctx context.Context, id string, inputs map[string]any) (*model.ExecutionContext, error) {
	if m.execFn != nil {
		return m.execFn(ctx, id, inputs)
	}
	return &model.ExecutionContext{RunID: "r1", WorkflowID: id}, nil
}
func (m *mockEngine) RegisterNodeExecutor(t model.NodeType, exec port.NodeExecutor) {
	if m.regExec != nil {
		m.regExec(t, exec)
	}
}

// newEngineTestHandler 构造带 mockEngine 的 Handler。
func newEngineTestHandler(e *mockEngine) *Handler {
	return New(nil, nil, e, nil, nil, &noopObserver{}, nil)
}

// TestRunWorkflow_OK 验证 RunWorkflow 正常返回 200。
func TestRunWorkflow_OK(t *testing.T) {
	eng := &mockEngine{
		execFn: func(_ context.Context, id string, inputs map[string]any) (*model.ExecutionContext, error) {
			assert.Equal(t, "wf1", id)
			assert.Equal(t, "v", inputs["k"])
			return &model.ExecutionContext{
				RunID:      "r-1",
				WorkflowID: id,
				Variables:  inputs,
			}, nil
		},
	}
	h := newEngineTestHandler(eng)
	r := newTestEngine("POST", "/api/v1/workflows/:id/run", h.RunWorkflow)

	body := `{"inputs":{"k":"v"}}`
	req := httptest.NewRequest("POST", "/api/v1/workflows/wf1/run", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "r-1", resp["RunID"])
}

// TestRunWorkflow_NotFound 验证工作流不存在返回 404。
func TestRunWorkflow_NotFound(t *testing.T) {
	eng := &mockEngine{
		execFn: func(_ context.Context, _ string, _ map[string]any) (*model.ExecutionContext, error) {
			return nil, domerrs.New(domerrs.NOT_FOUND, "workflow wf-x not found")
		},
	}
	h := newEngineTestHandler(eng)
	r := newTestEngine("POST", "/api/v1/workflows/:id/run", h.RunWorkflow)

	req := httptest.NewRequest("POST", "/api/v1/workflows/wf-x/run", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusNotFound, w.Code)
	var resp dtoErrorResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "NOT_FOUND", string(resp.Code))
}

// TestRunWorkflow_BadRequest 验证非法 body 返回 400。
func TestRunWorkflow_BadRequest(t *testing.T) {
	eng := &mockEngine{}
	h := newEngineTestHandler(eng)
	r := newTestEngine("POST", "/api/v1/workflows/:id/run", h.RunWorkflow)

	req := httptest.NewRequest("POST", "/api/v1/workflows/wf1/run", bytes.NewBufferString(`{"inputs":`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code)
}
