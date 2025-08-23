package cache

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"pc28-bot/internal/logger"
)

// MemoryItem 内存缓存项
type MemoryItem struct {
	Value     interface{}
	ExpiresAt time.Time
	CreatedAt time.Time
}

// IsExpired 检查是否过期
func (item *MemoryItem) IsExpired() bool {
	return time.Now().After(item.ExpiresAt)
}

// MemoryCache 内存缓存实现
type MemoryCache struct {
	items   sync.Map
	mutex   sync.RWMutex
	maxSize int
	size    int64
}

// NewMemoryCache 创建新的内存缓存
func NewMemoryCache(maxSize int) *MemoryCache {
	cache := &MemoryCache{
		maxSize: maxSize,
		size:    0,
	}

	// 启动清理协程
	go cache.startCleanup()

	logger.Info("Memory cache initialized")
	return cache
}

// Set 设置缓存值
func (m *MemoryCache) Set(key string, value interface{}, ttl time.Duration) error {
	item := &MemoryItem{
		Value:     value,
		ExpiresAt: time.Now().Add(ttl),
		CreatedAt: time.Now(),
	}

	// 检查缓存大小限制
	if m.size >= int64(m.maxSize) {
		m.evictOldest()
	}

	// 如果key已存在，则替换；否则增加计数
	if _, exists := m.items.LoadAndDelete(key); !exists {
		m.mutex.Lock()
		m.size++
		m.mutex.Unlock()
	}

	m.items.Store(key, item)
	logger.Debugf("Memory cache set: %s", key)
	return nil
}

// Get 获取缓存值
func (m *MemoryCache) Get(key string, dest interface{}) error {
	value, exists := m.items.Load(key)
	if !exists {
		return fmt.Errorf("cache miss: %s", key)
	}

	item := value.(*MemoryItem)
	if item.IsExpired() {
		m.items.Delete(key)
		m.mutex.Lock()
		m.size--
		m.mutex.Unlock()
		return fmt.Errorf("cache expired: %s", key)
	}

	// 使用JSON序列化/反序列化来复制数据，避免引用问题
	data, err := json.Marshal(item.Value)
	if err != nil {
		return fmt.Errorf("failed to marshal cache value: %v", err)
	}

	err = json.Unmarshal(data, dest)
	if err != nil {
		return fmt.Errorf("failed to unmarshal cache value: %v", err)
	}

	logger.Debugf("Memory cache hit: %s", key)
	return nil
}

// Delete 删除缓存
func (m *MemoryCache) Delete(key string) error {
	if _, exists := m.items.LoadAndDelete(key); exists {
		m.mutex.Lock()
		m.size--
		m.mutex.Unlock()
		logger.Debugf("Memory cache deleted: %s", key)
	}
	return nil
}

// DeletePattern 删除匹配模式的缓存
func (m *MemoryCache) DeletePattern(pattern string) error {
	var keysToDelete []string
	
	m.items.Range(func(key, value interface{}) bool {
		keyStr := key.(string)
		if matched, _ := matchPattern(pattern, keyStr); matched {
			keysToDelete = append(keysToDelete, keyStr)
		}
		return true
	})

	count := 0
	for _, key := range keysToDelete {
		if _, exists := m.items.LoadAndDelete(key); exists {
			count++
		}
	}

	if count > 0 {
		m.mutex.Lock()
		m.size -= int64(count)
		m.mutex.Unlock()
		logger.Debugf("Memory cache deleted by pattern: %s, count: %d", pattern, count)
	}

	return nil
}

// Exists 检查缓存是否存在
func (m *MemoryCache) Exists(key string) (bool, error) {
	value, exists := m.items.Load(key)
	if !exists {
		return false, nil
	}

	item := value.(*MemoryItem)
	if item.IsExpired() {
		m.items.Delete(key)
		m.mutex.Lock()
		m.size--
		m.mutex.Unlock()
		return false, nil
	}

	return true, nil
}

// SetTTL 设置缓存过期时间（内存缓存中重新设置值）
func (m *MemoryCache) SetTTL(key string, ttl time.Duration) error {
	value, exists := m.items.Load(key)
	if !exists {
		return fmt.Errorf("cache key not found: %s", key)
	}

	item := value.(*MemoryItem)
	item.ExpiresAt = time.Now().Add(ttl)
	m.items.Store(key, item)

	logger.Debugf("Memory cache TTL set: %s, ttl: %v", key, ttl)
	return nil
}

// GetTTL 获取缓存剩余过期时间
func (m *MemoryCache) GetTTL(key string) (time.Duration, error) {
	value, exists := m.items.Load(key)
	if !exists {
		return 0, fmt.Errorf("cache key not found: %s", key)
	}

	item := value.(*MemoryItem)
	if item.IsExpired() {
		return 0, nil
	}

	return time.Until(item.ExpiresAt), nil
}

// Clear 清空所有缓存
func (m *MemoryCache) Clear() {
	m.items = sync.Map{}
	m.mutex.Lock()
	m.size = 0
	m.mutex.Unlock()
	logger.Debug("Memory cache cleared")
}

// Size 获取缓存大小
func (m *MemoryCache) Size() int64 {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	return m.size
}

// Stats 获取缓存统计信息
func (m *MemoryCache) Stats() map[string]interface{} {
	m.mutex.RLock()
	size := m.size
	m.mutex.RUnlock()

	var validItems, expiredItems int64
	m.items.Range(func(key, value interface{}) bool {
		item := value.(*MemoryItem)
		if item.IsExpired() {
			expiredItems++
		} else {
			validItems++
		}
		return true
	})

	return map[string]interface{}{
		"total_size":    size,
		"valid_items":   validItems,
		"expired_items": expiredItems,
		"max_size":      m.maxSize,
	}
}

// startCleanup 启动定期清理过期缓存
func (m *MemoryCache) startCleanup() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		m.cleanupExpired()
	}
}

// cleanupExpired 清理过期的缓存项
func (m *MemoryCache) cleanupExpired() {
	var expiredKeys []interface{}

	m.items.Range(func(key, value interface{}) bool {
		item := value.(*MemoryItem)
		if item.IsExpired() {
			expiredKeys = append(expiredKeys, key)
		}
		return true
	})

	count := 0
	for _, key := range expiredKeys {
		if _, exists := m.items.LoadAndDelete(key); exists {
			count++
		}
	}

	if count > 0 {
		m.mutex.Lock()
		m.size -= int64(count)
		m.mutex.Unlock()
		logger.Debugf("Memory cache cleanup: removed %d expired items", count)
	}
}

// evictOldest 淘汰最旧的缓存项
func (m *MemoryCache) evictOldest() {
	var oldestKey interface{}
	var oldestTime time.Time

	m.items.Range(func(key, value interface{}) bool {
		item := value.(*MemoryItem)
		if oldestKey == nil || item.CreatedAt.Before(oldestTime) {
			oldestKey = key
			oldestTime = item.CreatedAt
		}
		return true
	})

	if oldestKey != nil {
		if _, exists := m.items.LoadAndDelete(oldestKey); exists {
			m.mutex.Lock()
			m.size--
			m.mutex.Unlock()
			logger.Debugf("Memory cache evicted oldest: %v", oldestKey)
		}
	}
}

// matchPattern 简单的模式匹配函数
func matchPattern(pattern, str string) (bool, error) {
	// 简单实现：支持 * 通配符
	if pattern == "*" {
		return true, nil
	}

	// 如果包含 * 在末尾，匹配前缀
	if len(pattern) > 0 && pattern[len(pattern)-1] == '*' {
		prefix := pattern[:len(pattern)-1]
		return len(str) >= len(prefix) && str[:len(prefix)] == prefix, nil
	}

	// 精确匹配
	return pattern == str, nil
}

