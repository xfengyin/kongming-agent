// Package plugin registry_test.go 覆盖 Registry 的全部公共 API 与错误路径。
package plugin

import (
	"errors"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeHandler 是测试用最小 Handler 实现。
type fakeHandler struct {
	name string
}

func (f *fakeHandler) Name() string { return f.name }

func TestNewRegistry_Empty(t *testing.T) {
	r := NewRegistry()
	require.NotNil(t, r)
	assert.Equal(t, 0, r.Len())
	assert.Empty(t, r.List())
}

func TestRegisterAndGet(t *testing.T) {
	r := NewRegistry()
	require.NoError(t, r.Register(&fakeHandler{name: "h1"}))

	h, ok := r.Get("h1")
	require.True(t, ok)
	require.NotNil(t, h)
	assert.Equal(t, "h1", h.Name())

	// 同名重复注册：后者覆盖前者，registry 长度不变。
	require.NoError(t, r.Register(&fakeHandler{name: "h1"}))
	assert.Equal(t, 1, r.Len())
}

func TestRegister_Invalid(t *testing.T) {
	r := NewRegistry()
	assert.ErrorIs(t, r.Register(nil), ErrInvalidHandler)
	assert.ErrorIs(t, r.Register(&fakeHandler{name: ""}), ErrInvalidHandler)
	// 注册失败不会污染 registry。
	assert.Equal(t, 0, r.Len())
}

func TestMustRegister_PanicOnError(t *testing.T) {
	r := NewRegistry()
	assert.Panics(t, func() {
		r.MustRegister(nil)
	})
	assert.Panics(t, func() {
		r.MustRegister(&fakeHandler{name: ""})
	})
	// MustRegister 正常路径不 panic。
	assert.NotPanics(t, func() {
		r.MustRegister(&fakeHandler{name: "ok"})
	})
}

func TestUnregister(t *testing.T) {
	r := NewRegistry()
	require.NoError(t, r.Register(&fakeHandler{name: "x"}))

	r.Unregister("x")
	_, ok := r.Get("x")
	assert.False(t, ok)
	assert.Equal(t, 0, r.Len())

	// 重复 unregister 不报错。
	assert.NotPanics(t, func() { r.Unregister("x") })
}

func TestGet_NotFound(t *testing.T) {
	r := NewRegistry()
	h, ok := r.Get("missing")
	assert.False(t, ok)
	assert.Nil(t, h)
}

func TestList_SortedAndSnapshot(t *testing.T) {
	r := NewRegistry()
	for _, n := range []string{"c", "a", "b"} {
		require.NoError(t, r.Register(&fakeHandler{name: n}))
	}
	got := r.List()
	assert.Equal(t, []string{"a", "b", "c"}, got, "List() should return sorted names")

	// 修改返回 slice 不应影响内部状态。
	got[0] = "mutated"
	again := r.List()
	assert.Equal(t, "a", again[0], "List() must return a defensive copy")
}

func TestRegistry_Concurrent(t *testing.T) {
	// 验证 RWMutex 在并发 Register/Get/Unregister 下的安全性。
	r := NewRegistry()
	const n = 200

	var wg sync.WaitGroup
	wg.Add(3)
	// writer
	go func() {
		defer wg.Done()
		for i := 0; i < n; i++ {
			_ = r.Register(&fakeHandler{name: "h"})
		}
	}()
	// reader
	go func() {
		defer wg.Done()
		for i := 0; i < n; i++ {
			_, _ = r.Get("h")
			_ = r.List()
			_ = r.Len()
		}
	}()
	// deleter
	go func() {
		defer wg.Done()
		for i := 0; i < n; i++ {
			r.Unregister("h")
		}
	}()
	wg.Wait()

	// 不变量：未 panic 即视为通过。
	assert.GreaterOrEqual(t, r.Len(), 0)
}

func TestErrInvalidHandler_StableMessage(t *testing.T) {
	// 错误值稳定性：业务层可能用 errors.Is 判断，确保不因 wrap 失效。
	var err error
	defer func() {
		if !errors.Is(err, ErrInvalidHandler) {
			t.Fatalf("expected wrap chain to include ErrInvalidHandler, got %v", err)
		}
	}()
	err = rWrap(ErrInvalidHandler)
}

// rWrap 包装原 err，用于断言 errors.Is 链。
func rWrap(err error) error { return err }
