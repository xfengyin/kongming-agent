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

	"github.com/zhuge/kongming/pkg/domain/errors"
	"github.com/zhuge/kongming/pkg/domain/model"
)

// mockVault 实现 port.Vault，便于 vault handler 单测。
type mockVault struct {
	listFn     func() ([]*model.Jinnang, error)
	executeFn  func(ctx context.Context, id string, input model.JinnangInput) (*model.JinnangOutput, error)
	getFn      func(id string) (*model.Jinnang, error)
	registerFn func(j *model.Jinnang, h model.JinnangHandler) error
	loadFn     func(ctx context.Context, dir string) error
}

func (m *mockVault) RegisterSkill(j *model.Jinnang, h model.JinnangHandler) error {
	if m.registerFn != nil {
		return m.registerFn(j, h)
	}
	return nil
}
func (m *mockVault) GetJinnang(id string) (*model.Jinnang, error) {
	if m.getFn != nil {
		return m.getFn(id)
	}
	return nil, nil
}
func (m *mockVault) ListJinnang() ([]*model.Jinnang, error) {
	if m.listFn != nil {
		return m.listFn()
	}
	return nil, nil
}
func (m *mockVault) Execute(ctx context.Context, id string, input model.JinnangInput) (*model.JinnangOutput, error) {
	if m.executeFn != nil {
		return m.executeFn(ctx, id, input)
	}
	return nil, nil
}
func (m *mockVault) LoadFromDir(ctx context.Context, dir string) error {
	if m.loadFn != nil {
		return m.loadFn(ctx, dir)
	}
	return nil
}

// newVaultTestHandler 构造带 mockVault 的 Handler。
func newVaultTestHandler(v *mockVault) *Handler {
	return New(nil, nil, nil, nil, v, &noopObserver{}, nil)
}

// TestListJinnang_OK 验证 ListJinnang 正常返回 200。
func TestListJinnang_OK(t *testing.T) {
	v := &mockVault{
		listFn: func() ([]*model.Jinnang, error) {
			return []*model.Jinnang{
				{ID: "j1", Name: "火攻", Type: model.JinnangTool},
				{ID: "j2", Name: "空城", Type: model.JinnangWisdom},
			}, nil
		},
	}
	h := newVaultTestHandler(v)
	r := newTestEngine("GET", "/api/v1/vault", h.ListJinnang)

	req := httptest.NewRequest("GET", "/api/v1/vault", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	items, ok := resp["jinnang"].([]any)
	require.True(t, ok)
	assert.Len(t, items, 2)
}

// TestExecuteJinnang_NotFound 验证锦囊不存在时返回 404。
func TestExecuteJinnang_NotFound(t *testing.T) {
	v := &mockVault{
		executeFn: func(_ context.Context, _ string, _ model.JinnangInput) (*model.JinnangOutput, error) {
			return nil, errors.New(errors.NOT_FOUND, "jinnang ghost not found")
		},
	}
	h := newVaultTestHandler(v)
	r := newTestEngine("POST", "/api/v1/vault/:id/exec", h.ExecuteJinnang)

	body := `{"params":{"k":"v"},"data":"payload"}`
	req := httptest.NewRequest("POST", "/api/v1/vault/ghost/exec", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusNotFound, w.Code)
	var resp dtoErrorResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "NOT_FOUND", string(resp.Code))
}

// TestExecuteJinnang_OK 验证正常执行返回 200。
func TestExecuteJinnang_OK(t *testing.T) {
	v := &mockVault{
		executeFn: func(_ context.Context, id string, in model.JinnangInput) (*model.JinnangOutput, error) {
			assert.Equal(t, "j1", id)
			assert.Equal(t, "payload", in.Data)
			return &model.JinnangOutput{Success: true, Data: "ok"}, nil
		},
	}
	h := newVaultTestHandler(v)
	r := newTestEngine("POST", "/api/v1/vault/:id/exec", h.ExecuteJinnang)

	body := `{"data":"payload"}`
	req := httptest.NewRequest("POST", "/api/v1/vault/j1/exec", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, true, resp["Success"])
}

// TestExecuteJinnang_BadRequest 验证 body 非法 JSON 时返回 400。
func TestExecuteJinnang_BadRequest(t *testing.T) {
	v := &mockVault{}
	h := newVaultTestHandler(v)
	r := newTestEngine("POST", "/api/v1/vault/:id/exec", h.ExecuteJinnang)

	req := httptest.NewRequest("POST", "/api/v1/vault/j1/exec", bytes.NewBufferString(`{"data":`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code)
}
