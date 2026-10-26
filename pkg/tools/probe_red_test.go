package tools

import "testing"

// 探针：故意失败，验证触发段改动之后必需检查仍然生效。验完即删。
func TestProbeGateStillWorks(t *testing.T) {
	t.Fatal("探针：这是故意的失败")
}
