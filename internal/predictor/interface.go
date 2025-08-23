package predictor

import (
	"fmt"
	"pc28-bot/internal/database"
)

// Predictor 预测算法接口
type Predictor interface {
	// Predict 根据历史数据进行预测
	Predict(history []database.LotteryResult) (*database.PredictionResult, error)
	
	// GetName 获取算法名称
	GetName() string
	
	// GetVersion 获取算法版本
	GetVersion() string
	
	// ValidateInput 验证输入数据
	ValidateInput(history []database.LotteryResult) error
	
	// GetRequiredHistorySize 获取所需的历史数据大小
	GetRequiredHistorySize() int
}

// PredictorManager 预测器管理器
type PredictorManager struct {
	predictors map[string]Predictor
	current    Predictor
}

// NewPredictorManager 创建新的预测器管理器
func NewPredictorManager() *PredictorManager {
	manager := &PredictorManager{
		predictors: make(map[string]Predictor),
	}
	
	// 注册默认预测器
	defaultPredictor := NewDefaultPredictor()
	manager.RegisterPredictor(defaultPredictor)
	manager.SetCurrentPredictor("default")
	
	return manager
}

// RegisterPredictor 注册预测器
func (pm *PredictorManager) RegisterPredictor(predictor Predictor) {
	pm.predictors[predictor.GetName()] = predictor
}

// SetCurrentPredictor 设置当前预测器
func (pm *PredictorManager) SetCurrentPredictor(name string) error {
	predictor, exists := pm.predictors[name]
	if !exists {
		return fmt.Errorf("predictor not found: %s", name)
	}
	pm.current = predictor
	return nil
}

// GetCurrentPredictor 获取当前预测器
func (pm *PredictorManager) GetCurrentPredictor() Predictor {
	return pm.current
}

// GetAvailablePredictors 获取可用的预测器列表
func (pm *PredictorManager) GetAvailablePredictors() []string {
	var names []string
	for name := range pm.predictors {
		names = append(names, name)
	}
	return names
}

// Predict 使用当前预测器进行预测
func (pm *PredictorManager) Predict(history []database.LotteryResult) (*database.PredictionResult, error) {
	if pm.current == nil {
		return nil, fmt.Errorf("no current predictor set")
	}
	return pm.current.Predict(history)
}
