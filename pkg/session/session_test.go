// 会话持久化测试

package session

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/xfengyin/kongming-agent/pkg/llm"
)

func testHistory() []llm.Message {
	return []llm.Message{
		{Role: llm.RoleUser, Content: "第一问：天下大势如何？"},
		{Role: llm.RoleAssistant, Content: "亮答：分久必合。"},
		{Role: llm.RoleUser, Content: "第二问：如何北伐？"},
	}
}

func TestSessionSaveLoad(t *testing.T) {
	path := filepath.Join(t.TempDir(), "session.json")
	s := New("./knowledge", testHistory())

	if err := s.Save(path); err != nil {
		t.Fatalf("Save 失败: %v", err)
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load 失败: %v", err)
	}
	if loaded.Version != Version {
		t.Errorf("版本不匹配: %d", loaded.Version)
	}
	if loaded.KnowledgeDir != "./knowledge" {
		t.Errorf("knowledge 配置丢失: %q", loaded.KnowledgeDir)
	}
	if len(loaded.History) != 3 {
		t.Fatalf("历史条数不匹配: %d", len(loaded.History))
	}
	if loaded.History[0].Content != "第一问：天下大势如何？" {
		t.Errorf("历史内容还原错误: %+v", loaded.History[0])
	}
	if loaded.History[1].Role != llm.RoleAssistant {
		t.Errorf("角色还原错误: %+v", loaded.History[1])
	}
	if loaded.CreatedAt.IsZero() || loaded.UpdatedAt.IsZero() {
		t.Errorf("时间戳缺失")
	}
}

func TestSessionLoadNonexistent(t *testing.T) {
	_, err := Load(filepath.Join(t.TempDir(), "no-such.json"))
	if err == nil {
		t.Fatal("加载不存在的文件应报错")
	}
	if !strings.Contains(err.Error(), "读取会话文件失败") {
		t.Errorf("错误信息不明确: %v", err)
	}
}

func TestSessionLoadCorrupt(t *testing.T) {
	path := filepath.Join(t.TempDir(), "corrupt.json")
	if err := os.WriteFile(path, []byte("{not valid json!!!"), 0o644); err != nil {
		t.Fatalf("写损坏文件失败: %v", err)
	}
	_, err := Load(path)
	if err == nil {
		t.Fatal("损坏文件应报错")
	}
	if !strings.Contains(err.Error(), "损坏") {
		t.Errorf("错误信息应提示损坏: %v", err)
	}
}

func TestSessionLoadWrongVersion(t *testing.T) {
	path := filepath.Join(t.TempDir(), "old-version.json")
	data := `{"version":999,"created_at":"2026-01-01T00:00:00Z","updated_at":"2026-01-01T00:00:00Z","history":[]}`
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatalf("写文件失败: %v", err)
	}
	_, err := Load(path)
	if err == nil {
		t.Fatal("版本不兼容应报错")
	}
	if !strings.Contains(err.Error(), "版本不兼容") {
		t.Errorf("错误信息应提示版本不兼容: %v", err)
	}
}

func TestSessionSaveNil(t *testing.T) {
	var s *Session
	if err := s.Save(filepath.Join(t.TempDir(), "x.json")); err == nil {
		t.Fatal("nil 会话 Save 应报错")
	}
}

func TestSessionSaveLoadRoundTripWithHistoryRestore(t *testing.T) {
	path := filepath.Join(t.TempDir(), "roundtrip.json")
	s := New("", testHistory())
	if err := s.Save(path); err != nil {
		t.Fatalf("Save 失败: %v", err)
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load 失败: %v", err)
	}
	// 还原为消息切片交给 agent.LoadHistory 继续对话（cmd/kongming --load 的用法）
	h := loaded.History
	if len(h) != 3 {
		t.Errorf("还原 history 条数错误: %d", len(h))
	}
	// 模拟继续对话：追加一轮
	h = append(h, llm.Message{Role: llm.RoleUser, Content: "第三问"})
	if len(h) != 4 {
		t.Errorf("继续对话后条数错误: %d", len(h))
	}
}

func TestSessionSaveUpdatesTimestamp(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ts.json")
	s := New("", nil)
	s.CreatedAt = time.Now().Add(-time.Hour)
	if err := s.Save(path); err != nil {
		t.Fatalf("Save 失败: %v", err)
	}
	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load 失败: %v", err)
	}
	if !loaded.UpdatedAt.After(loaded.CreatedAt) {
		t.Errorf("UpdatedAt 应晚于 CreatedAt: created=%v updated=%v", loaded.CreatedAt, loaded.UpdatedAt)
	}
}
