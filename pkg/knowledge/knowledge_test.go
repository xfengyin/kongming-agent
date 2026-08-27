// 轻量知识库测试

package knowledge

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// sanguoFixture 测试用三国典故（段落间以空行分隔）
const sanguoFixture = `# 隆中对

刘备三顾茅庐，诸葛亮提出隆中对：先取荆州为家，再取益州成鼎足之势，
然后待天下有变，两路北伐以图中原。

# 空城计

司马懿大军压境，诸葛亮大开城门，焚香抚琴。司马懿疑有伏兵，退兵而去。

# 草船借箭

诸葛亮立军令状三日造十万支箭，趁大雾以草船诱敌，曹军射箭如雨，
尽收箭支，不费吹灰之力。`

// writeTempMD 在临时目录写 .md 文件，返回目录
func writeTempMD(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatalf("写临时文件失败: %v", err)
		}
	}
	return dir
}

func TestLoadAndCount(t *testing.T) {
	dir := writeTempMD(t, map[string]string{"sanguo.md": sanguoFixture})
	store, err := Load(dir)
	if err != nil {
		t.Fatalf("Load 失败: %v", err)
	}
	if store.Count() != 3 {
		t.Errorf("期望 3 个段落，实际 %d", store.Count())
	}
}

func TestLoadSkipsNonMD(t *testing.T) {
	dir := writeTempMD(t, map[string]string{
		"sanguo.md":  "# 测试\n内容",
		"notes.txt":  "不是 md",
		"ignore.md~": "备份文件",
	})
	store, err := Load(dir)
	if err != nil {
		t.Fatalf("Load 失败: %v", err)
	}
	if store.Count() != 1 {
		t.Errorf("只应加载 .md 文件，实际 %d 个段落", store.Count())
	}
}

func TestLoadEmptyDir(t *testing.T) {
	dir := t.TempDir() // 空目录
	store, err := Load(dir)
	if err != nil {
		t.Fatalf("空目录 Load 失败: %v", err)
	}
	if store.Count() != 0 {
		t.Errorf("空目录应 0 段落，实际 %d", store.Count())
	}
	if got := store.Search("天下", 3); len(got) != 0 {
		t.Errorf("空库检索应为空，实际 %d 条", len(got))
	}
}

func TestLoadNonexistentDir(t *testing.T) {
	_, err := Load(filepath.Join(t.TempDir(), "no-such-dir"))
	if err == nil {
		t.Fatal("不存在的目录应报错")
	}
}

func TestSearchHit(t *testing.T) {
	dir := writeTempMD(t, map[string]string{"sanguo.md": sanguoFixture})
	store, err := Load(dir)
	if err != nil {
		t.Fatalf("Load 失败: %v", err)
	}

	got := store.Search("草船借箭", 1)
	if len(got) != 1 {
		t.Fatalf("期望命中 1 条，实际 %d", len(got))
	}
	if !strings.Contains(got[0].Content, "草船借箭") {
		t.Errorf("命中段落应包含查询词，实际: %s", got[0].Content[:30])
	}
}

func TestSearchRankingAndLimit(t *testing.T) {
	dir := writeTempMD(t, map[string]string{"sanguo.md": sanguoFixture})
	store, err := Load(dir)
	if err != nil {
		t.Fatalf("Load 失败: %v", err)
	}

	// "荆州" 命中隆中对，应排第一且限 1 条
	got := store.Search("荆州", 1)
	if len(got) != 1 {
		t.Fatalf("limit=1 应返回 1 条，实际 %d", len(got))
	}
	if got[0].Title != "隆中对" {
		t.Errorf("期望隆中对居首，实际 %q", got[0].Title)
	}

	// "借箭" 命中草船借箭，应居首
	got2 := store.Search("借箭", 5)
	if len(got2) < 1 {
		t.Errorf("借箭应至少命中草船借箭")
	}
	if got2[0].Title != "草船借箭" {
		t.Errorf("期望草船借箭居首，实际 %q", got2[0].Title)
	}
}

func TestSearchNoMatch(t *testing.T) {
	dir := writeTempMD(t, map[string]string{"sanguo.md": sanguoFixture})
	store, err := Load(dir)
	if err != nil {
		t.Fatalf("Load 失败: %v", err)
	}
	got := store.Search("量子计算", 3)
	if len(got) != 0 {
		t.Errorf("无匹配应返回 0 条，实际 %d", len(got))
	}
}

func TestLoadCRLFLineEndings(t *testing.T) {
	// Windows CRLF 文件：\n\n 切分失效曾导致整文件塌成 1 段
	crlf := strings.ReplaceAll(sanguoFixture, "\n", "\r\n")
	dir := writeTempMD(t, map[string]string{"sanguo.md": crlf})
	store, err := Load(dir)
	if err != nil {
		t.Fatalf("Load 失败: %v", err)
	}
	if store.Count() != 3 {
		t.Errorf("CRLF 文件应切出 3 个段落，实际 %d", store.Count())
	}
	// 检索应正常命中
	got := store.Search("草船借箭", 1)
	if len(got) != 1 {
		t.Errorf("CRLF 文件检索应命中草船借箭，实际 %d 条", len(got))
	}
}
