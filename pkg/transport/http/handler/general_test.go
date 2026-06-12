package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	domerrs "github.com/zhuge/kongming/pkg/domain/errors"
	"github.com/zhuge/kongming/pkg/domain/model"
)

// mockPool 实现 port.GeneralPool。
type mockPool struct {
	listFn    func(ctx context.Context) ([]*model.General, error)
	getFn     func(ctx context.Context, id model.GeneralID) (*model.General, error)
	regFn     func(ctx context.Context, g *model.General) error
	unregFn   func(ctx context.Context, id model.GeneralID) error
	selBestFn func(skill string) (*model.General, error)
	execFn    func(ctx context.Context, id model.GeneralID, o *model.Order) (*model.GeneralReport, error)
}

func (m *mockPool) Get(ctx context.Context, id model.GeneralID) (*model.General, error) {
	if m.getFn != nil {
		return m.getFn(ctx, id)
	}
	return nil, nil
}
func (m *mockPool) List(ctx context.Context) ([]*model.General, error) {
	if m.listFn != nil {
		return m.listFn(ctx)
	}
	return nil, nil
}
func (m *mockPool) Register(ctx context.Context, g *model.General) error {
	if m.regFn != nil {
		return m.regFn(ctx, g)
	}
	return nil
}
func (m *mockPool) Unregister(ctx context.Context, id model.GeneralID) error {
	if m.unregFn != nil {
		return m.unregFn(ctx, id)
	}
	return nil
}
func (m *mockPool) SelectBest(skill string) (*model.General, error) {
	if m.selBestFn != nil {
		return m.selBestFn(skill)
	}
	return nil, nil
}
func (m *mockPool) Execute(ctx context.Context, id model.GeneralID, o *model.Order) (*model.GeneralReport, error) {
	if m.execFn != nil {
		return m.execFn(ctx, id, o)
	}
	return nil, nil
}

// newPoolTestHandler 构造带 mockPool 的 Handler。
func newPoolTestHandler(p *mockPool) *Handler {
	return New(nil, nil, nil, p, nil, &noopObserver{}, nil)
}

// TestListGenerals_OK 验证 ListGenerals 正常返回 200。
func TestListGenerals_OK(t *testing.T) {
	p := &mockPool{
		listFn: func(_ context.Context) ([]*model.General, error) {
			return []*model.General{
				{ID: model.GeneralID("g1"), Name: "关羽", Type: model.GeneralGuanYu},
			}, nil
		},
	}
	h := newPoolTestHandler(p)
	r := newTestEngine("GET", "/api/v1/generals", h.ListGenerals)

	req := httptest.NewRequest("GET", "/api/v1/generals", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	items, ok := resp["generals"].([]any)
	require.True(t, ok)
	assert.Len(t, items, 1)
}

// TestGetGeneral_NotFound 验证将领不存在返回 404。
func TestGetGeneral_NotFound(t *testing.T) {
	p := &mockPool{
		getFn: func(_ context.Context, _ model.GeneralID) (*model.General, error) {
			return nil, domerrs.New(domerrs.NOT_FOUND, "general g1 not found")
		},
	}
	h := newPoolTestHandler(p)
	r := newTestEngine("GET", "/api/v1/generals/:id", h.GetGeneral)

	req := httptest.NewRequest("GET", "/api/v1/generals/g1", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusNotFound, w.Code)
	var resp dtoErrorResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "NOT_FOUND", string(resp.Code))
}

// TestGetGeneral_InternalError 验证非 *domerrs.Error 错误归一化为 INTERNAL。
func TestGetGeneral_InternalError(t *testing.T) {
	p := &mockPool{
		getFn: func(_ context.Context, _ model.GeneralID) (*model.General, error) {
			return nil, errors.New("plain error")
		},
	}
	h := newPoolTestHandler(p)
	r := newTestEngine("GET", "/api/v1/generals/:id", h.GetGeneral)

	req := httptest.NewRequest("GET", "/api/v1/generals/g1", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusInternalServerError, w.Code)
	var resp dtoErrorResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "INTERNAL", string(resp.Code))
}
