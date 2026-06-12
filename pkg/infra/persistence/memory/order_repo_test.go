// OrderRepo 单元测试：覆盖 Save / Get / List / Delete 全部公开方法
// 以及错误分支（nil/空 ID/不存在 key）。
package memory

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/zhuge/kongming/pkg/domain/model"
)

// TestOrderRepo_SaveAndGet 验证 Save 后能 Get 到相同对象。
func TestOrderRepo_SaveAndGet(t *testing.T) {
	repo := NewOrderRepo(NewStore())
	ctx := context.Background()

	o := &model.Order{
		ID:        "o1",
		Name:      "test-order",
		State:     model.StatePending,
		CreatedAt: time.Now().UTC(),
	}
	if err := repo.Save(ctx, o); err != nil {
		t.Fatalf("save: %v", err)
	}

	got, err := repo.Get(ctx, "o1")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Name != "test-order" {
		t.Errorf("name: want %q, got %q", "test-order", got.Name)
	}
	if got.State != model.StatePending {
		t.Errorf("state: want %v, got %v", model.StatePending, got.State)
	}
	if !got.CreatedAt.Equal(o.CreatedAt) {
		t.Errorf("createdAt: want %v, got %v", o.CreatedAt, got.CreatedAt)
	}
}

// TestOrderRepo_GetMissing 验证查询不存在的 ID 返回 ErrOrderNotFound。
func TestOrderRepo_GetMissing(t *testing.T) {
	repo := NewOrderRepo(NewStore())
	_, err := repo.Get(context.Background(), "nope")
	if err == nil {
		t.Fatal("want error, got nil")
	}
	if !errors.Is(err, ErrOrderNotFound) {
		t.Errorf("want ErrOrderNotFound, got %v", err)
	}
}

// TestOrderRepo_ListByState 验证按 state 过滤；StateNone 返回全量。
func TestOrderRepo_ListByState(t *testing.T) {
	repo := NewOrderRepo(NewStore())
	ctx := context.Background()

	// 准备数据：2 个 Pending + 1 个 Completed。
	seed := []*model.Order{
		{ID: "a", Name: "a", State: model.StatePending},
		{ID: "b", Name: "b", State: model.StatePending},
		{ID: "c", Name: "c", State: model.StateCompleted},
	}
	for _, o := range seed {
		if err := repo.Save(ctx, o); err != nil {
			t.Fatalf("save %s: %v", o.ID, err)
		}
	}

	// 过滤 Pending：期望 2 个。
	got, err := repo.List(ctx, model.StatePending)
	if err != nil {
		t.Fatalf("list pending: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("pending count: want 2, got %d", len(got))
	}

	// 过滤 Completed：期望 1 个。
	got, err = repo.List(ctx, model.StateCompleted)
	if err != nil {
		t.Fatalf("list completed: %v", err)
	}
	if len(got) != 1 {
		t.Errorf("completed count: want 1, got %d", len(got))
	}
	if got[0].ID != model.OrderID("c") {
		t.Errorf("completed id: want c, got %s", got[0].ID)
	}

	// state==StateNone：返回全量。
	got, err = repo.List(ctx, model.StateNone)
	if err != nil {
		t.Fatalf("list all: %v", err)
	}
	if len(got) != 3 {
		t.Errorf("all count: want 3, got %d", len(got))
	}
}

// TestOrderRepo_Delete 验证删除后 Get 不到，重复删除幂等。
func TestOrderRepo_Delete(t *testing.T) {
	repo := NewOrderRepo(NewStore())
	ctx := context.Background()

	if err := repo.Save(ctx, &model.Order{ID: "x", Name: "x"}); err != nil {
		t.Fatalf("save: %v", err)
	}
	if err := repo.Delete(ctx, "x"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := repo.Get(ctx, "x"); !errors.Is(err, ErrOrderNotFound) {
		t.Errorf("after delete: want ErrOrderNotFound, got %v", err)
	}
	// 重复删除不应报错（幂等）。
	if err := repo.Delete(ctx, "x"); err != nil {
		t.Errorf("re-delete: want nil, got %v", err)
	}
}

// TestOrderRepo_Save_Validation 验证参数校验：nil 订单 / 空 ID 应报错。
func TestOrderRepo_Save_Validation(t *testing.T) {
	repo := NewOrderRepo(NewStore())
	ctx := context.Background()

	if err := repo.Save(ctx, nil); err == nil {
		t.Error("save nil: want error, got nil")
	}
	if err := repo.Save(ctx, &model.Order{ID: ""}); err == nil {
		t.Error("save empty id: want error, got nil")
	}
}

// TestOrderRepo_Get_EmptyID 验证空 ID 的防御。
func TestOrderRepo_Get_EmptyID(t *testing.T) {
	repo := NewOrderRepo(NewStore())
	if _, err := repo.Get(context.Background(), ""); err == nil {
		t.Error("get empty id: want error, got nil")
	}
}

// TestOrderRepo_Delete_EmptyID 验证空 ID 的 Delete 防御。
func TestOrderRepo_Delete_EmptyID(t *testing.T) {
	repo := NewOrderRepo(NewStore())
	if err := repo.Delete(context.Background(), ""); err == nil {
		t.Error("delete empty id: want error, got nil")
	}
}

// TestOrderRepo_ConcurrentSafe 验证在 -race 下并发 Save/Get 不会数据竞争。
func TestOrderRepo_ConcurrentSafe(t *testing.T) {
	repo := NewOrderRepo(NewStore())
	ctx := context.Background()

	var wg sync.WaitGroup
	const n = 200
	for i := 0; i < n; i++ {
		wg.Add(2)
		go func(i int) {
			defer wg.Done()
			o := &model.Order{ID: model.OrderID(string(rune('a' + i%26))), Name: "x", State: model.StatePending}
			_ = repo.Save(ctx, o)
		}(i)
		go func(i int) {
			defer wg.Done()
			_, _ = repo.Get(ctx, model.OrderID(string(rune('a'+i%26))))
		}(i)
	}
	wg.Wait()

	// 最终应当能枚举到至少 1 条记录（最多 26 个唯一 ID）。
	got, err := repo.List(ctx, model.StateNone)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(got) == 0 {
		t.Error("want at least 1 order after concurrent writes, got 0")
	}
}

// TestOrderRepo_NewOrderRepo_NilStore 验证 NewOrderRepo(nil) 触发 panic。
func TestOrderRepo_NewOrderRepo_NilStore(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("want panic on nil Store, got none")
		}
	}()
	_ = NewOrderRepo(nil)
}

// TestOrderRepo_ImplementsPort 编译期已断言，这里再加一个运行期 sanity check。
func TestOrderRepo_ImplementsPort(t *testing.T) {
	var _ interface {
		Save(ctx context.Context, o *model.Order) error
		Get(ctx context.Context, id model.OrderID) (*model.Order, error)
		List(ctx context.Context, state model.State) ([]*model.Order, error)
		Delete(ctx context.Context, id model.OrderID) error
	} = NewOrderRepo(NewStore())
}
