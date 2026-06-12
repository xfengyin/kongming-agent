package observability

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewTraceIDContext(t *testing.T) {
	ctx := NewTraceIDContext(context.Background(), "abc-123")
	assert.Equal(t, "abc-123", FromTraceIDContext(ctx))
}

func TestFromEmptyContext(t *testing.T) {
	assert.Equal(t, "", FromTraceIDContext(context.Background()))
}

func TestNewTraceID(t *testing.T) {
	id1 := NewTraceID()
	id2 := NewTraceID()
	assert.NotEqual(t, id1, id2)
	assert.NotEmpty(t, id1)
}
