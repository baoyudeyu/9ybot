package cache

import (
	"fmt"
	"time"

	"pc28-bot/internal/database"
	"pc28-bot/internal/logger"
)

// CacheManager 内存缓存管理器
type CacheManager struct {
	memory     *MemoryCache
	mysql      *database.MySQLDB
	defaultTTL time.Duration
}

// NewCacheManager 创建新的缓存管理器
func NewCacheManager(mysql *database.MySQLDB, defaultTTL time.Duration) (*CacheManager, error) {
	// 初始化内存缓存
	memoryCache := NewMemoryCache(1000) // 最大1000项

	manager := &CacheManager{
		memory:     memoryCache,
		mysql:      mysql,
		defaultTTL: defaultTTL,
	}

	logger.Info("Cache manager initialized with Memory + MySQL")
	return manager, nil
}

// Close 关闭缓存管理器
func (cm *CacheManager) Close() error {
	logger.Info("Cache manager closed")
	return nil
}

// Get 获取缓存数据
func (cm *CacheManager) Get(key string, dest interface{}) error {
	// 尝试从内存缓存获取
	err := cm.memory.Get(key, dest)
	if err == nil {
		return nil
	}

	// 从数据库获取（根据不同的缓存键类型）
	data, err := cm.getFromDatabase(key)
	if err != nil {
		return fmt.Errorf("cache miss: %s", key)
	}

	// 回填到内存缓存
	cm.memory.Set(key, data, cm.defaultTTL)

	// 将数据复制到目标对象
	return cm.copyData(data, dest)
}

// Set 设置缓存数据
func (cm *CacheManager) Set(key string, value interface{}, ttl time.Duration) error {
	if ttl == 0 {
		ttl = cm.defaultTTL
	}

	// 写入内存缓存
	if err := cm.memory.Set(key, value, ttl); err != nil {
		logger.Warnf("Failed to set memory cache: %v", err)
		return err
	}

	logger.Debugf("Cache set: %s", key)
	return nil
}

// Delete 删除缓存
func (cm *CacheManager) Delete(key string) error {
	cm.memory.Delete(key)
	logger.Debugf("Cache deleted: %s", key)
	return nil
}

// InvalidatePattern 按模式失效缓存
func (cm *CacheManager) InvalidatePattern(pattern string) error {
	cm.memory.DeletePattern(pattern)
	logger.Debugf("Cache invalidated by pattern: %s", pattern)
	return nil
}

// OnNewLotteryData 新开奖数据事件处理
func (cm *CacheManager) OnNewLotteryData(data *database.LotteryResult) error {
	logger.Info("Processing cache update for new lottery data")

	// 失效相关缓存
	cm.InvalidatePattern("lottery:*")
	cm.InvalidatePattern("prediction:*")

	// 更新最新数据缓存
	cm.Set("lottery:latest", data, cm.defaultTTL)

	// 获取并缓存最近3期数据
	last3, err := cm.mysql.GetLatestLotteryResults(3)
	if err == nil {
		cm.Set("lottery:last3", last3, cm.defaultTTL)
	}

	// 获取并缓存最新10期数据
	last10, err := cm.mysql.GetLatestLotteryResults(10)
	if err == nil {
		cm.Set("lottery:last10", last10, cm.defaultTTL)
	}

	logger.Infof("Cache updated for new lottery data: %s", data.Qihao)
	return nil
}

// OnPredictionGenerated 预测生成事件处理
func (cm *CacheManager) OnPredictionGenerated(prediction *database.Prediction) error {
	logger.Info("Processing cache update for new prediction")

	// 失效预测相关缓存
	cm.InvalidatePattern("prediction:*")
	cm.InvalidatePattern("stats:*")

	// 更新最新预测缓存
	cm.Set("prediction:latest", prediction, cm.defaultTTL)

	// 获取并缓存最近10期预测记录
	last10Predictions, err := cm.mysql.GetLatestPredictions(10)
	if err == nil {
		cm.Set("prediction:history:10", last10Predictions, cm.defaultTTL)
	}

	logger.Infof("Cache updated for new prediction: %s", prediction.TargetQihao)
	return nil
}

// OnPredictionVerified 预测验证事件处理
func (cm *CacheManager) OnPredictionVerified(qihao string, isCorrect bool) error {
	logger.Info("Processing cache update for prediction verification")

	// 失效统计相关缓存
	cm.InvalidatePattern("stats:*")
	cm.InvalidatePattern("prediction:history:*")

	// 获取并缓存更新的统计数据
	stats, err := cm.mysql.GetPredictionStats()
	if err == nil {
		cm.Set("stats:accuracy", stats, cm.defaultTTL)
	}

	// 更新预测历史缓存
	last10Predictions, err := cm.mysql.GetLatestPredictions(10)
	if err == nil {
		cm.Set("prediction:history:10", last10Predictions, cm.defaultTTL)
	}

	logger.Infof("Cache updated for prediction verification: %s, correct: %t", qihao, isCorrect)
	return nil
}

// GetLatestLotteryData 获取最新开奖数据
func (cm *CacheManager) GetLatestLotteryData() (*database.LotteryResult, error) {
	var result database.LotteryResult
	err := cm.Get("lottery:latest", &result)
	if err != nil {
		// 从数据库获取
		results, err := cm.mysql.GetLatestLotteryResults(1)
		if err != nil || len(results) == 0 {
			return nil, fmt.Errorf("no lottery data found")
		}
		result = results[0]
		cm.Set("lottery:latest", result, cm.defaultTTL)
	}
	return &result, nil
}

// GetLast3LotteryData 获取最近3期开奖数据
func (cm *CacheManager) GetLast3LotteryData() ([]database.LotteryResult, error) {
	var results []database.LotteryResult
	err := cm.Get("lottery:last3", &results)
	if err != nil {
		// 从数据库获取
		results, err = cm.mysql.GetLatestLotteryResults(3)
		if err != nil {
			return nil, err
		}
		cm.Set("lottery:last3", results, cm.defaultTTL)
	}
	return results, nil
}

// GetLatestPrediction 获取最新预测
func (cm *CacheManager) GetLatestPrediction() (*database.Prediction, error) {
	var prediction database.Prediction
	err := cm.Get("prediction:latest", &prediction)
	if err != nil {
		// 从数据库获取
		predictions, err := cm.mysql.GetLatestPredictions(1)
		if err != nil || len(predictions) == 0 {
			return nil, fmt.Errorf("no prediction found")
		}
		prediction = predictions[0]
		cm.Set("prediction:latest", prediction, cm.defaultTTL)
	}
	return &prediction, nil
}

// GetLotteryHistory 获取历史开奖数据
func (cm *CacheManager) GetLotteryHistory(limit int) ([]database.LotteryResult, error) {
	cacheKey := fmt.Sprintf("lottery:history:%d", limit)
	var history []database.LotteryResult

	// 尝试从内存缓存获取
	if err := cm.memory.Get(cacheKey, &history); err == nil {
		return history, nil
	}

	// 从数据库获取
	history, err := cm.mysql.GetLotteryHistory(limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get lottery history from database: %v", err)
	}

	// 保存到内存缓存
	cm.memory.Set(cacheKey, history, 5*time.Minute)

	return history, nil
}

// GetPredictionHistory 获取预测历史
func (cm *CacheManager) GetPredictionHistory(limit int) ([]database.Prediction, error) {
	cacheKey := fmt.Sprintf("prediction:history:%d", limit)
	var predictions []database.Prediction

	err := cm.Get(cacheKey, &predictions)
	if err != nil {
		// 从数据库获取
		predictions, err = cm.mysql.GetLatestPredictions(limit)
		if err != nil {
			return nil, err
		}
		cm.Set(cacheKey, predictions, cm.defaultTTL)
	}
	return predictions, nil
}

// GetPredictionStats 获取预测统计
func (cm *CacheManager) GetPredictionStats() (*database.PredictionStats, error) {
	var stats database.PredictionStats
	err := cm.Get("stats:accuracy", &stats)
	if err != nil {
		// 从数据库获取
		statsPtr, err := cm.mysql.GetPredictionStats()
		if err != nil {
			return nil, err
		}
		stats = *statsPtr
		cm.Set("stats:accuracy", stats, cm.defaultTTL)
	}
	return &stats, nil
}

// getFromDatabase 根据缓存键从数据库获取数据
func (cm *CacheManager) getFromDatabase(key string) (interface{}, error) {
	switch key {
	case "lottery:latest":
		results, err := cm.mysql.GetLatestLotteryResults(1)
		if err != nil || len(results) == 0 {
			return nil, fmt.Errorf("no latest lottery data")
		}
		return results[0], nil

	case "lottery:last3":
		return cm.mysql.GetLatestLotteryResults(3)

	case "lottery:last10":
		return cm.mysql.GetLatestLotteryResults(10)

	case "prediction:latest":
		predictions, err := cm.mysql.GetLatestPredictions(1)
		if err != nil || len(predictions) == 0 {
			return nil, fmt.Errorf("no latest prediction")
		}
		return predictions[0], nil

	case "prediction:history:10":
		return cm.mysql.GetLatestPredictions(10)

	case "stats:accuracy":
		return cm.mysql.GetPredictionStats()

	default:
		return nil, fmt.Errorf("unknown cache key: %s", key)
	}
}

// copyData 复制数据到目标对象
func (cm *CacheManager) copyData(src, dest interface{}) error {
	// 简单的类型断言复制
	switch v := src.(type) {
	case database.LotteryResult:
		if ptr, ok := dest.(*database.LotteryResult); ok {
			*ptr = v
			return nil
		}
	case []database.LotteryResult:
		if ptr, ok := dest.(*[]database.LotteryResult); ok {
			*ptr = v
			return nil
		}
	case database.Prediction:
		if ptr, ok := dest.(*database.Prediction); ok {
			*ptr = v
			return nil
		}
	case []database.Prediction:
		if ptr, ok := dest.(*[]database.Prediction); ok {
			*ptr = v
			return nil
		}
	case database.PredictionStats:
		if ptr, ok := dest.(*database.PredictionStats); ok {
			*ptr = v
			return nil
		}
	}
	return fmt.Errorf("unsupported type conversion")
}

// GetStats 获取缓存统计信息
func (cm *CacheManager) GetStats() map[string]interface{} {
	memStats := cm.memory.Stats()

	return map[string]interface{}{
		"memory_cache": memStats,
		"cache_layers": 2, // Memory + MySQL
	}
}
