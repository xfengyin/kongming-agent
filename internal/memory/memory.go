// Kongming 孔明军师系统 - 内存记忆实现
// 过目不忘，温故知新

package memory

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// MemoryType 记忆类型
type MemoryType string

const (
	MemoryShortTerm MemoryType = "short" // 短期记忆（会话级）
	MemoryMidTerm   MemoryType = "mid"   // 中期记忆（日/周级）
	MemoryLongTerm  MemoryType = "long"  // 长期记忆（永久）
)

// MemoryEntry 记忆条目
type MemoryEntry struct {
	ID          string                 `json:"id"`
	Type        MemoryType            `json:"type"`
	Key         string                 `json:"key"`
	Content     interface{}            `json:"content"`
	Tags        []string               `json:"tags"`
	Weight      float64               `json:"weight"` // 重要性权重
	AccessCount int                   `json:"access_count"`
	CreatedAt   time.Time             `json:"created_at"`
	UpdatedAt   time.Time             `json:"updated_at"`
	ExpiresAt   *time.Time            `json:"expires_at,omitempty"`
}

// SearchResult 搜索结果
type SearchResult struct {
	Entry *MemoryEntry `json:"entry"`
	Score float64     `json:"score"` // 相关性评分
}

// Memory 记忆系统接口
type Memory interface {
	// Store 存储记忆
	Store(ctx context.Context, entry *MemoryEntry) error

	// Retrieve 检索记忆
	Retrieve(ctx context.Context, key string) (*MemoryEntry, error)

	// Search 搜索记忆
	Search(ctx context.Context, query string, memType MemoryType, limit int) []*SearchResult

	// Forget 遗忘记忆
	Forget(ctx context.Context, key string) error

	// Consolidate 记忆整合（短期->中期->长期）
	Consolidate(ctx context.Context) error

	// GetRecent 获取最近的记忆
	GetRecent(memType MemoryType, limit int) []*MemoryEntry
}

// ZhugeMemory 诸葛记忆实现
type ZhugeMemory struct {
	mu          sync.RWMutex
	shortTerm   map[string]*MemoryEntry
	midTerm     map[string]*MemoryEntry
	longTerm    map[string]*MemoryEntry

	// 配置
	shortTermTTL time.Duration
	midTermTTL   time.Duration
}

// NewZhugeMemory 创建诸葛记忆
func NewZhugeMemory() *ZhugeMemory {
	m := &ZhugeMemory{
		shortTerm:    make(map[string]*MemoryEntry),
		midTerm:      make(map[string]*MemoryEntry),
		longTerm:     make(map[string]*MemoryEntry),
		shortTermTTL: 24 * time.Hour,
		midTermTTL:   7 * 24 * time.Hour,
	}

	// 启动清理协程
	go m.cleanupLoop()

	return m
}

// Store 存储记忆
func (m *ZhugeMemory) Store(ctx context.Context, entry *MemoryEntry) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if entry.ID == "" {
		entry.ID = generateMemoryID()
	}

	now := time.Now()
	entry.UpdatedAt = now
	if entry.CreatedAt.IsZero() {
		entry.CreatedAt = now
	}

	// 根据类型设置过期时间
	switch entry.Type {
	case MemoryShortTerm:
		expires := now.Add(m.shortTermTTL)
		entry.ExpiresAt = &expires
		m.shortTerm[entry.Key] = entry

	case MemoryMidTerm:
		expires := now.Add(m.midTermTTL)
		entry.ExpiresAt = &expires
		m.midTerm[entry.Key] = entry

	case MemoryLongTerm:
		entry.ExpiresAt = nil // 长期记忆不过期
		m.longTerm[entry.Key] = entry
	}

	return nil
}

// Retrieve 检索记忆
func (m *ZhugeMemory) Retrieve(ctx context.Context, key string) (*MemoryEntry, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 按优先级查找：短期 -> 中期 -> 长期
	if entry, exists := m.shortTerm[key]; exists {
		entry.AccessCount++
		return entry, nil
	}

	if entry, exists := m.midTerm[key]; exists {
		entry.AccessCount++
		return entry, nil
	}

	if entry, exists := m.longTerm[key]; exists {
		entry.AccessCount++
		return entry, nil
	}

	return nil, fmt.Errorf("记忆未找到: %s", key)
}

// Search 搜索记忆
func (m *ZhugeMemory) Search(ctx context.Context, query string, memType MemoryType, limit int) []*SearchResult {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var results []*SearchResult

	// 根据类型搜索
	switch memType {
	case MemoryShortTerm:
		results = m.searchInMap(m.shortTerm, query)
	case MemoryMidTerm:
		results = m.searchInMap(m.midTerm, query)
	case MemoryLongTerm:
		results = m.searchInMap(m.longTerm, query)
	default:
		// 搜索所有类型
		results = append(results, m.searchInMap(m.shortTerm, query)...)
		results = append(results, m.searchInMap(m.midTerm, query)...)
		results = append(results, m.searchInMap(m.longTerm, query)...)
	}

	// 按评分排序并限制数量
	if len(results) > limit && limit > 0 {
		results = results[:limit]
	}

	return results
}

// Forget 遗忘记忆
func (m *ZhugeMemory) Forget(ctx context.Context, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.shortTerm, key)
	delete(m.midTerm, key)
	delete(m.longTerm, key)

	return nil
}

// Consolidate 记忆整合
func (m *ZhugeMemory) Consolidate(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()

	// 短期 -> 中期：高频访问的短期记忆
	for key, entry := range m.shortTerm {
		if entry.AccessCount >= 3 && entry.Weight > 0.7 {
			entry.Type = MemoryMidTerm
			expires := now.Add(m.midTermTTL)
			entry.ExpiresAt = &expires
			m.midTerm[key] = entry
			delete(m.shortTerm, key)
		}
	}

	// 中期 -> 长期：高频访问且权重高的中期记忆
	for key, entry := range m.midTerm {
		if entry.AccessCount >= 10 && entry.Weight > 0.9 {
			entry.Type = MemoryLongTerm
			entry.ExpiresAt = nil
			m.longTerm[key] = entry
			delete(m.midTerm, key)
		}
	}

	return nil
}

// GetRecent 获取最近的记忆
func (m *ZhugeMemory) GetRecent(memType MemoryType, limit int) []*MemoryEntry {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var entries []*MemoryEntry

	switch memType {
	case MemoryShortTerm:
		for _, entry := range m.shortTerm {
			entries = append(entries, entry)
		}
	case MemoryMidTerm:
		for _, entry := range m.midTerm {
			entries = append(entries, entry)
		}
	case MemoryLongTerm:
		for _, entry := range m.longTerm {
			entries = append(entries, entry)
		}
	}

	// 按更新时间排序
	for i := 0; i < len(entries)-1; i++ {
		for j := i + 1; j < len(entries); j++ {
			if entries[j].UpdatedAt.After(entries[i].UpdatedAt) {
				entries[i], entries[j] = entries[j], entries[i]
			}
		}
	}

	if limit > 0 && len(entries) > limit {
		entries = entries[:limit]
	}

	return entries
}

// searchInMap 在map中搜索
func (m *ZhugeMemory) searchInMap(data map[string]*MemoryEntry, query string) []*SearchResult {
	var results []*SearchResult

	for _, entry := range data {
		score := m.calculateRelevance(entry, query)
		if score > 0 {
			results = append(results, &SearchResult{
				Entry: entry,
				Score: score,
			})
		}
	}

	return results
}

// calculateRelevance 计算相关性
func (m *ZhugeMemory) calculateRelevance(entry *MemoryEntry, query string) float64 {
	score := 0.0

	// 标签匹配
	for _, tag := range entry.Tags {
		if tag == query {
			score += 1.0
		}
	}

	// Key匹配
	if entry.Key == query {
		score += 0.8
	}

	// 内容匹配（简化实现）
	if content, ok := entry.Content.(string); ok {
		if len(content) > 0 && query != "" {
			score += 0.5
		}
		_ = content
	}

	// 权重加成
	score *= entry.Weight

	return score
}

// cleanupLoop 清理过期记忆
func (m *ZhugeMemory) cleanupLoop() {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for range ticker.C {
		m.cleanup()
	}
}

// cleanup 清理过期记忆
func (m *ZhugeMemory) cleanup() {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()

	// 清理短期记忆
	for key, entry := range m.shortTerm {
		if entry.ExpiresAt != nil && entry.ExpiresAt.Before(now) {
			delete(m.shortTerm, key)
		}
	}

	// 清理中期记忆
	for key, entry := range m.midTerm {
		if entry.ExpiresAt != nil && entry.ExpiresAt.Before(now) {
			delete(m.midTerm, key)
		}
	}
}

func generateMemoryID() string {
	return fmt.Sprintf("mem_%d", time.Now().UnixNano())
}
