package cache

import (
	"fmt"
	"sync"
	"time"

	"pc28-bot/internal/database"
	"pc28-bot/internal/logger"
)

// SimpleCache 简化的缓存实现（只使用内存缓存）
type SimpleCache struct {
	data   sync.Map
	mysql  *database.MySQLDB
	ttl    time.Duration
	mutex  sync.RWMutex
}

// CacheItem 缓存项
type CacheItem struct {
	Value     interface{}
	ExpiresAt time.Time
}

// IsExpired 检查是否过期
func (item *CacheItem) IsExpired() bool {
	return time.Now().After(item.ExpiresAt)
}

// NewSimpleCache 创建简化缓存
func NewSimpleCache(mysql *database.MySQLDB, ttl time.Duration) *SimpleCache {
	cache := &SimpleCache{
		mysql: mysql,
		ttl:   ttl,
	}
	
	// 启动清理协程
	go cache.cleanup()
	
	logger.Info("Simple cache initialized")
	return cache
}

// Get 获取缓存数据
func (sc *SimpleCache) Get(key string, dest interface{}) error {
	value, exists := sc.data.Load(key)
	if !exists {
		return sc.loadFromDatabase(key, dest)
	}

	item := value.(*CacheItem)
	if item.IsExpired() {
		sc.data.Delete(key)
		return sc.loadFromDatabase(key, dest)
	}

	// 复制数据到目标
	return sc.copyValue(item.Value, dest)
}

// Set 设置缓存数据
func (sc *SimpleCache) Set(key string, value interface{}) {
	item := &CacheItem{
		Value:     value,
		ExpiresAt: time.Now().Add(sc.ttl),
	}
	sc.data.Store(key, item)
}

// Delete 删除缓存
func (sc *SimpleCache) Delete(key string) {
	sc.data.Delete(key)
}

// Clear 清空缓存
func (sc *SimpleCache) Clear() {
	sc.data = sync.Map{}
}

// loadFromDatabase 从数据库加载数据
func (sc *SimpleCache) loadFromDatabase(key string, dest interface{}) error {
	switch key {
	case "latest_lottery":
		results, err := sc.mysql.GetLatestLotteryResults(1)
		if err != nil || len(results) == 0 {
			return fmt.Errorf("no latest lottery data")
		}
		sc.Set(key, results[0])
		return sc.copyValue(results[0], dest)

	case "last3_lottery":
		results, err := sc.mysql.GetLatestLotteryResults(3)
		if err != nil {
			return err
		}
		sc.Set(key, results)
		return sc.copyValue(results, dest)

	case "latest_prediction":
		predictions, err := sc.mysql.GetLatestPredictions(1)
		if err != nil || len(predictions) == 0 {
			return fmt.Errorf("no latest prediction")
		}
		sc.Set(key, predictions[0])
		return sc.copyValue(predictions[0], dest)

	case "prediction_history":
		predictions, err := sc.mysql.GetLatestPredictions(10)
		if err != nil {
			return err
		}
		sc.Set(key, predictions)
		return sc.copyValue(predictions, dest)

	case "prediction_stats":
		stats, err := sc.mysql.GetPredictionStats()
		if err != nil {
			return err
		}
		sc.Set(key, *stats)
		return sc.copyValue(*stats, dest)

	default:
		return fmt.Errorf("unknown cache key: %s", key)
	}
}

// copyValue 复制值
func (sc *SimpleCache) copyValue(src, dest interface{}) error {
	switch d := dest.(type) {
	case *database.LotteryResult:
		if s, ok := src.(database.LotteryResult); ok {
			*d = s
			return nil
		}
	case *[]database.LotteryResult:
		if s, ok := src.([]database.LotteryResult); ok {
			*d = s
			return nil
		}
	case *database.Prediction:
		if s, ok := src.(database.Prediction); ok {
			*d = s
			return nil
		}
	case *[]database.Prediction:
		if s, ok := src.([]database.Prediction); ok {
			*d = s
			return nil
		}
	case *database.PredictionStats:
		if s, ok := src.(database.PredictionStats); ok {
			*d = s
			return nil
		}
	}
	return fmt.Errorf("type mismatch in cache copy")
}

// cleanup 定期清理过期缓存
func (sc *SimpleCache) cleanup() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		sc.data.Range(func(key, value interface{}) bool {
			if item := value.(*CacheItem); item.IsExpired() {
				sc.data.Delete(key)
			}
			return true
		})
	}
}

// OnNewLotteryData 新开奖数据事件
func (sc *SimpleCache) OnNewLotteryData(data *database.LotteryResult) {
	sc.Delete("latest_lottery")
	sc.Delete("last3_lottery")
	sc.Set("latest_lottery", *data)
}

// OnPredictionGenerated 新预测生成事件
func (sc *SimpleCache) OnPredictionGenerated(prediction *database.Prediction) {
	sc.Delete("latest_prediction")
	sc.Delete("prediction_history")
	sc.Set("latest_prediction", *prediction)
}

// OnPredictionVerified 预测验证事件
func (sc *SimpleCache) OnPredictionVerified() {
	sc.Delete("prediction_stats")
	sc.Delete("prediction_history")
}

