// 轻量本地知识库（RAG 最简形态）
// 学而时习之，不亦说乎
//
// 设计约束（见 docs/architecture 合理限制文档）：
//   - 仅读取本地 .md 文件，按段落（空行分隔）切分；
//   - 检索用简单「词频 + 包含匹配」，不引入向量库/外部依赖；
//   - 定位是「读本地文件塞进上下文」的最简 RAG，不做索引持久化。

package knowledge

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

// Paragraph 知识段落
type Paragraph struct {
	File    string // 来源文件（相对加载目录）
	Title   string // 段落标题（# 开头的行，无则为空）
	Content string // 段落正文
}

// Store 本地知识库
type Store struct {
	mu         sync.RWMutex
	paragraphs []Paragraph
	dir        string
}

// Load 读取 dir 目录下所有 .md 文件（非递归），按段落切分
func Load(dir string) (*Store, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("读取知识库目录失败: %w", err)
	}

	s := &Store{dir: dir}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if !strings.HasSuffix(strings.ToLower(e.Name()), ".md") {
			continue
		}
		path := filepath.Join(dir, e.Name())
		content, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("读取知识文件失败 %s: %w", path, err)
		}
		s.paragraphs = append(s.paragraphs, splitParagraphs(e.Name(), string(content))...)
	}
	return s, nil
}

// Dir 加载目录（用于提示）
func (s *Store) Dir() string { return s.dir }

// Count 段落总数
func (s *Store) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.paragraphs)
}

// Search 用词频 + 包含匹配检索相关段落，返回最相关的 limit 条
func (s *Store) Search(query string, limit int) []Paragraph {
	if limit <= 0 {
		limit = 3
	}
	terms := tokenize(query)

	s.mu.RLock()
	defer s.mu.RUnlock()

	type scored struct {
		p     Paragraph
		score float64
	}
	results := make([]scored, 0, len(s.paragraphs))
	for _, p := range s.paragraphs {
		score := scoreParagraph(p, terms)
		if score > 0 {
			results = append(results, scored{p: p, score: score})
		}
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].score > results[j].score
	})
	if len(results) > limit {
		results = results[:limit]
	}

	out := make([]Paragraph, 0, len(results))
	for _, r := range results {
		out = append(out, r.p)
	}
	return out
}

// splitParagraphs 按空行切分 .md 内容为段落。
// 若某块只有标题行（# 开头）且下一块是正文，则合并为一个段落（标题+正文）。
func splitParagraphs(file, content string) []Paragraph {
	blocks := strings.Split(content, "\n\n")
	paras := make([]Paragraph, 0, len(blocks))

	for i := 0; i < len(blocks); i++ {
		trimmed := strings.TrimSpace(blocks[i])
		if trimmed == "" {
			continue
		}
		title := extractTitle(trimmed)
		body := trimmed

		// 标题块 + 下一块正文 → 合并
		if isTitleOnly(trimmed) && i+1 < len(blocks) {
			next := strings.TrimSpace(blocks[i+1])
			if next != "" && extractTitle(next) == "" {
				body = trimmed + "\n\n" + next
				i++ // 消费下一块
			}
		}

		paras = append(paras, Paragraph{
			File:    file,
			Title:   title,
			Content: body,
		})
	}
	return paras
}

// extractTitle 取块内首个 # 标题行文本
func extractTitle(block string) string {
	for _, line := range strings.Split(block, "\n") {
		l := strings.TrimSpace(line)
		if strings.HasPrefix(l, "#") {
			return strings.TrimSpace(strings.TrimLeft(l, "# "))
		}
	}
	return ""
}

// isTitleOnly 块是否只含标题行（无正文）
func isTitleOnly(block string) bool {
	for _, line := range strings.Split(block, "\n") {
		l := strings.TrimSpace(line)
		if l == "" {
			continue
		}
		if !strings.HasPrefix(l, "#") {
			return false
		}
	}
	return true
}

// tokenize 中文按字符二元组 + 英文按词切分，用于轻量匹配
func tokenize(s string) []string {
	s = strings.ToLower(s)
	var tokens []string
	// 中文：连续 CJK 片段按 2-gram
	var cjk []rune
	flush := func() {
		if len(cjk) == 1 {
			tokens = append(tokens, string(cjk))
		} else {
			for i := 0; i+1 < len(cjk); i++ {
				tokens = append(tokens, string(cjk[i:i+2]))
			}
		}
		cjk = cjk[:0]
	}
	for _, r := range s {
		if r >= 0x4e00 && r <= 0x9fff { // CJK 统一表意文字
			cjk = append(cjk, r)
		} else {
			flush()
		}
	}
	flush()
	// 英文/数字：按空白分词
	for _, w := range strings.Fields(s) {
		w = strings.Trim(w, ".,;:!?()[]\"'")
		if w != "" {
			tokens = append(tokens, w)
		}
	}
	return tokens
}

// scoreParagraph 段落得分：命中词数 + 标题命中加权
func scoreParagraph(p Paragraph, terms []string) float64 {
	content := strings.ToLower(p.Content)
	title := strings.ToLower(p.Title)
	score := 0.0
	for _, t := range terms {
		if t == "" {
			continue
		}
		if strings.Contains(content, t) {
			score += 1.0
		}
		if strings.Contains(title, t) {
			score += 2.0
		}
	}
	return score
}
