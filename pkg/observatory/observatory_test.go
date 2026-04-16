// Kongming 观测台测试

package observatory

import (
	"context"
	"testing"
	"time"
)

func TestObservatoryStart(t *testing.T) {
	obs := NewObservatory()
	ctx := context.Background()

	err := obs.Start(ctx)
	if err != nil {
		t.Errorf("启动观测台失败: %v", err)
	}
}

func TestRecordHTTPRequest(t *testing.T) {
	RecordHTTPRequest("GET", "/api/test", 200, 100*time.Millisecond)
}

func TestSetActiveOrders(t *testing.T) {
	SetActiveOrders(10)
}

func TestRecordTaskProcessed(t *testing.T) {
	RecordTaskProcessed("success")
	RecordTaskProcessed("failed")
}
