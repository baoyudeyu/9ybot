package predictor

import (
	"fmt"
	"math/rand"
	"time"

	"pc28-bot/internal/database"
	"pc28-bot/internal/logger"
)

// DefaultPredictor 默认预测算法实现
type DefaultPredictor struct {
	name    string
	version string
}

// NewDefaultPredictor 创建默认预测器
func NewDefaultPredictor() *DefaultPredictor {
	return &DefaultPredictor{
		name:    "default",
		version: "v1.0",
	}
}

// GetName 获取算法名称
func (dp *DefaultPredictor) GetName() string {
	return dp.name
}

// GetVersion 获取算法版本
func (dp *DefaultPredictor) GetVersion() string {
	return dp.version
}

// GetRequiredHistorySize 获取所需的历史数据大小
func (dp *DefaultPredictor) GetRequiredHistorySize() int {
	return 3 // 需要最近3期数据
}

// ValidateInput 验证输入数据
func (dp *DefaultPredictor) ValidateInput(history []database.LotteryResult) error {
	if len(history) < dp.GetRequiredHistorySize() {
		return fmt.Errorf("insufficient history data: need %d, got %d",
			dp.GetRequiredHistorySize(), len(history))
	}

	// 验证数据完整性
	for i, result := range history {
		if result.Qihao == "" {
			return fmt.Errorf("empty qihao in history[%d]", i)
		}
		if result.OpenNum == "" {
			return fmt.Errorf("empty opennum in history[%d]", i)
		}

		// 验证开奖号码格式
		_, err := database.ParseOpenNum(result.OpenNum)
		if err != nil {
			return fmt.Errorf("invalid opennum format in history[%d]: %v", i, err)
		}
	}

	return nil
}

// Predict 根据历史数据进行预测
func (dp *DefaultPredictor) Predict(history []database.LotteryResult) (*database.PredictionResult, error) {
	// 验证输入
	if err := dp.ValidateInput(history); err != nil {
		return nil, err
	}

	// 获取最近3期数据进行分析
	recent3 := history[:3]
	logger.Debugf("Analyzing recent 3 periods for prediction")

	// 使用固定算法进行预测
	predictedNums := dp.analyzeAndPredict(recent3)

	// 生成下一期期号
	nextQihao := dp.generateNextQihao(recent3[0].Qihao)

	result := &database.PredictionResult{
		TargetQihao:      nextQihao,
		PredictedNum:     database.FormatOpenNum(predictedNums),
		ConfidenceScore:  0.0, // 移除置信度逻辑
		AlgorithmVersion: dp.GetVersion(),
		Timestamp:        time.Now(),
	}

	logger.Infof("Prediction generated: %s -> %s (fixed algorithm)",
		nextQihao, result.PredictedNum)

	return result, nil
}

// analyzeAndPredict 分析历史数据并进行预测（固定算法）
func (dp *DefaultPredictor) analyzeAndPredict(history []database.LotteryResult) []int {
	logger.Debug("Using fixed algorithm for prediction")

	// 解析历史数据的开奖号码
	var allNums [][]int
	for _, result := range history {
		nums, err := database.ParseOpenNum(result.OpenNum)
		if err != nil {
			logger.Warnf("Failed to parse opennum %s: %v", result.OpenNum, err)
			continue
		}
		allNums = append(allNums, nums)
	}

	// 使用固定算法进行预测
	return dp.fixedOddEvenPrediction(allNums)
}

// fixedOddEvenPrediction 固定单双预测算法
func (dp *DefaultPredictor) fixedOddEvenPrediction(allNums [][]int) []int {
	if len(allNums) < 3 {
		logger.Warn("Insufficient data for fixed prediction, using default")
		return []int{1, 2, 3} // 默认预测
	}

	// 按照用户提供的算法逻辑：
	// 最新3期的每个位置相加，然后求平均值，根据平均值的单双性进行预测
	// 例如：3326098期的4 & 3326099期的6 & 3326100期的1 =11
	//      3326098期的0 & 3326099期的6 & 3326100期的7 =13
	//      3326098期的3 & 3326099期的6 & 3326100期的9 =18
	//      11+13+18=42, 42/3=14=双，预测为双

	positionSums := make([]int, 3)

	// 计算每个位置的和（注意：allNums[0]是最新的，allNums[2]是最老的）
	for pos := 0; pos < 3; pos++ {
		sum := 0
		for i := 0; i < 3; i++ {
			if pos < len(allNums[i]) {
				sum += allNums[i][pos]
			}
		}
		positionSums[pos] = sum
	}

	// 计算总和并求平均值
	totalSum := positionSums[0] + positionSums[1] + positionSums[2]
	average := totalSum / 3

	logger.Debugf("Algorithm calculation: pos_sums=%v, total=%d, avg=%d",
		positionSums, totalSum, average)

	// 根据平均值的单双性生成预测号码
	// 这里生成的是具体的号码，但主要用于计算和值的单双性
	predicted := make([]int, 3)
	if average%2 == 0 {
		// 平均值是双，预测双数和值
		// 生成一个和值为双数的号码组合
		predicted[0] = 2
		predicted[1] = 4
		predicted[2] = 0 // 2+4+0=6 (双)
	} else {
		// 平均值是单，预测单数和值
		// 生成一个和值为单数的号码组合
		predicted[0] = 1
		predicted[1] = 3
		predicted[2] = 1 // 1+3+1=5 (单)
	}

	logger.Debugf("Prediction result: predicted_nums=%v, predicted_sum=%d, odd_even=%s",
		predicted, database.CalculateSum(predicted),
		database.CalculateOddEven(database.CalculateSum(predicted)))

	return predicted
}

// 移除了复杂的频率预测算法，只保留固定算法

// 移除了所有置信度和智能预测相关函数

// generateNextQihao 生成下一期期号
func (dp *DefaultPredictor) generateNextQihao(latestQihao string) string {
	// PC28期号格式通常是7位数字，如：3326106
	if len(latestQihao) >= 7 {
		// 尝试直接解析整个期号为数字
		var qihaoNum int
		if _, err := fmt.Sscanf(latestQihao, "%d", &qihaoNum); err == nil {
			return fmt.Sprintf("%d", qihaoNum+1)
		}

		// 如果上述方法失败，尝试分割前缀和后缀
		if len(latestQihao) == 7 {
			prefix := latestQihao[:4] // 前4位
			numStr := latestQihao[4:] // 后3位

			var num int
			if _, err := fmt.Sscanf(numStr, "%d", &num); err == nil {
				return fmt.Sprintf("%s%03d", prefix, num+1)
			}
		}
	}

	// 如果解析失败，返回默认值
	logger.Warnf("Failed to parse qihao: %s, using default", latestQihao)
	return "3326999"
}

// GetPredictionSummary 获取预测摘要信息（简化版）
func (dp *DefaultPredictor) GetPredictionSummary(history []database.LotteryResult) map[string]interface{} {
	return map[string]interface{}{
		"algorithm":      dp.GetName(),
		"version":        dp.GetVersion(),
		"history_size":   len(history),
		"required_size":  dp.GetRequiredHistorySize(),
		"analysis_ready": len(history) >= dp.GetRequiredHistorySize(),
		"method":         "fixed_algorithm",
	}
}

func init() {
	// 初始化随机数种子
	rand.Seed(time.Now().UnixNano())
}

