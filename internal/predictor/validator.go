package predictor

import (
	"fmt"
	"time"

	"pc28-bot/internal/database"
	"pc28-bot/internal/logger"
)

// ValidationResult 验证结果
type ValidationResult struct {
	IsCorrect        bool      `json:"is_correct"`
	MatchType        string    `json:"match_type"`        // "exact", "partial", "none"
	MatchedPositions []int     `json:"matched_positions"` // 匹配的位置
	PredictedNumbers []int     `json:"predicted_numbers"`
	ActualNumbers    []int     `json:"actual_numbers"`
	PredictedSum     int       `json:"predicted_sum"`
	ActualSum        int       `json:"actual_sum"`
	ValidationTime   time.Time `json:"validation_time"`
}

// Validator 预测验证器
type Validator struct {
	mysql *database.MySQLDB
}

// NewValidator 创建新的验证器
func NewValidator(mysql *database.MySQLDB) *Validator {
	return &Validator{
		mysql: mysql,
	}
}

// ValidatePrediction 验证预测结果
func (v *Validator) ValidatePrediction(qihao string, actualResult *database.LotteryResult) (*ValidationResult, error) {
	logger.Infof("Validating prediction for qihao: %s", qihao)

	// 从数据库获取对应的预测记录
	predictions, err := v.mysql.GetLatestPredictions(20) // 获取最近20条预测记录
	if err != nil {
		return nil, fmt.Errorf("failed to get predictions: %v", err)
	}

	var targetPrediction *database.Prediction
	for _, pred := range predictions {
		if pred.TargetQihao == qihao {
			targetPrediction = &pred
			break
		}
	}

	if targetPrediction == nil {
		return nil, fmt.Errorf("no prediction found for qihao: %s", qihao)
	}

	// 解析预测号码和实际号码
	predictedNums, err := database.ParseOpenNum(targetPrediction.PredictedNum)
	if err != nil {
		return nil, fmt.Errorf("failed to parse predicted numbers: %v", err)
	}

	actualNums, err := database.ParseOpenNum(actualResult.OpenNum)
	if err != nil {
		return nil, fmt.Errorf("failed to parse actual numbers: %v", err)
	}

	// 进行详细验证
	result := v.performDetailedValidation(predictedNums, actualNums)
	result.ValidationTime = time.Now()

	// 使用数据库的ValidatePrediction方法来更新记录
	isCorrect, err := v.mysql.ValidatePrediction(qihao, actualResult)
	if err != nil {
		logger.Errorf("Failed to validate prediction in database: %v", err)
	} else {
		result.IsCorrect = isCorrect
	}

	logger.Infof("Prediction validation completed for %s: %s (match: %s)",
		qihao, map[bool]string{true: "CORRECT", false: "INCORRECT"}[result.IsCorrect], result.MatchType)

	return result, nil
}

// performDetailedValidation 执行详细验证
func (v *Validator) performDetailedValidation(predicted, actual []int) *ValidationResult {
	result := &ValidationResult{
		PredictedNumbers: predicted,
		ActualNumbers:    actual,
		PredictedSum:     database.CalculateSum(predicted),
		ActualSum:        database.CalculateSum(actual),
	}

	// 检查完全匹配
	if v.isExactMatch(predicted, actual) {
		result.IsCorrect = true
		result.MatchType = "exact"
		result.MatchedPositions = []int{0, 1, 2}
		return result
	}

	// 检查部分匹配
	matchedPositions := v.getMatchedPositions(predicted, actual)
	if len(matchedPositions) > 0 {
		result.MatchType = "partial"
		result.MatchedPositions = matchedPositions
		// 部分匹配也可以根据策略定义为正确或错误
		result.IsCorrect = len(matchedPositions) >= 2 // 至少匹配2个位置算正确
	} else {
		result.MatchType = "none"
		result.IsCorrect = false
	}

	return result
}

// isExactMatch 检查是否完全匹配
func (v *Validator) isExactMatch(predicted, actual []int) bool {
	if len(predicted) != len(actual) {
		return false
	}

	for i := range predicted {
		if predicted[i] != actual[i] {
			return false
		}
	}

	return true
}

// getMatchedPositions 获取匹配的位置
func (v *Validator) getMatchedPositions(predicted, actual []int) []int {
	var matched []int

	minLen := len(predicted)
	if len(actual) < minLen {
		minLen = len(actual)
	}

	for i := 0; i < minLen; i++ {
		if predicted[i] == actual[i] {
			matched = append(matched, i)
		}
	}

	return matched
}

// ValidateBatch 批量验证预测结果
func (v *Validator) ValidateBatch(results []database.LotteryResult) ([]ValidationResult, error) {
	var validationResults []ValidationResult

	for _, result := range results {
		validation, err := v.ValidatePrediction(result.Qihao, &result)
		if err != nil {
			logger.Warnf("Failed to validate prediction for %s: %v", result.Qihao, err)
			continue
		}
		validationResults = append(validationResults, *validation)
	}

	logger.Infof("Batch validation completed: %d results processed", len(validationResults))
	return validationResults, nil
}

// Statistics 统计信息
type Statistics struct {
	TotalPredictions     int       `json:"total_predictions"`
	CorrectPredictions   int       `json:"correct_predictions"`
	IncorrectPredictions int       `json:"incorrect_predictions"`
	AccuracyRate         float64   `json:"accuracy_rate"`
	ExactMatches         int       `json:"exact_matches"`
	PartialMatches       int       `json:"partial_matches"`
	NoMatches            int       `json:"no_matches"`
	AverageConfidence    float64   `json:"average_confidence"`
	LastUpdateTime       time.Time `json:"last_update_time"`
}

// StatisticsCalculator 统计计算器
type StatisticsCalculator struct {
	mysql *database.MySQLDB
}

// NewStatisticsCalculator 创建统计计算器
func NewStatisticsCalculator(mysql *database.MySQLDB) *StatisticsCalculator {
	return &StatisticsCalculator{
		mysql: mysql,
	}
}

// CalculateStatistics 计算统计信息
func (sc *StatisticsCalculator) CalculateStatistics() (*Statistics, error) {
	logger.Debug("Calculating prediction statistics")

	// 从数据库获取统计信息
	dbStats, err := sc.mysql.GetPredictionStats()
	if err != nil {
		return nil, fmt.Errorf("failed to get prediction stats from database: %v", err)
	}

	// 获取详细的预测记录进行深度分析
	predictions, err := sc.mysql.GetLatestPredictions(100) // 分析最近100条记录
	if err != nil {
		return nil, fmt.Errorf("failed to get prediction details: %v", err)
	}

	stats := &Statistics{
		TotalPredictions:     dbStats.TotalPredictions,
		CorrectPredictions:   dbStats.CorrectPredictions,
		IncorrectPredictions: dbStats.TotalPredictions - dbStats.CorrectPredictions,
		AccuracyRate:         dbStats.AccuracyRate,
		LastUpdateTime:       time.Now(),
	}

	// 计算详细统计
	sc.calculateDetailedStats(predictions, stats)

	logger.Infof("Statistics calculated: accuracy=%.2f%%, total=%d",
		stats.AccuracyRate, stats.TotalPredictions)

	return stats, nil
}

// calculateDetailedStats 计算详细统计
func (sc *StatisticsCalculator) calculateDetailedStats(predictions []database.Prediction, stats *Statistics) {
	var confidenceSum float64
	validConfidenceCount := 0

	for _, pred := range predictions {
		// 计算平均置信度
		if pred.ConfidenceScore != nil {
			confidenceSum += *pred.ConfidenceScore
			validConfidenceCount++
		}

		// 如果预测已验证，进行详细分类
		if pred.IsCorrect != nil && pred.ActualNum != nil {
			if err := sc.categorizeMatch(&pred, stats); err != nil {
				logger.Warnf("Failed to categorize match for prediction %d: %v", pred.ID, err)
			}
		}
	}

	// 计算平均置信度
	if validConfidenceCount > 0 {
		stats.AverageConfidence = confidenceSum / float64(validConfidenceCount)
	}
}

// categorizeMatch 分类匹配类型
func (sc *StatisticsCalculator) categorizeMatch(pred *database.Prediction, stats *Statistics) error {
	if pred.ActualNum == nil {
		return fmt.Errorf("actual number is nil")
	}

	predictedNums, err := database.ParseOpenNum(pred.PredictedNum)
	if err != nil {
		return fmt.Errorf("failed to parse predicted numbers: %v", err)
	}

	actualNums, err := database.ParseOpenNum(*pred.ActualNum)
	if err != nil {
		return fmt.Errorf("failed to parse actual numbers: %v", err)
	}

	// 检查匹配类型
	if len(predictedNums) == 3 && len(actualNums) == 3 {
		exactMatch := true
		matchCount := 0

		for i := 0; i < 3; i++ {
			if predictedNums[i] == actualNums[i] {
				matchCount++
			} else {
				exactMatch = false
			}
		}

		if exactMatch {
			stats.ExactMatches++
		} else if matchCount > 0 {
			stats.PartialMatches++
		} else {
			stats.NoMatches++
		}
	}

	return nil
}

// GetPerformanceReport 获取性能报告
func (sc *StatisticsCalculator) GetPerformanceReport(days int) (map[string]interface{}, error) {
	// 获取指定天数的预测记录
	predictions, err := sc.mysql.GetLatestPredictions(days * 288) // PC28每天约288期
	if err != nil {
		return nil, fmt.Errorf("failed to get predictions for performance report: %v", err)
	}

	// 按天分组统计
	dailyStats := make(map[string]map[string]int)

	for _, pred := range predictions {
		dateKey := pred.PredictedAt.Format("2006-01-02")

		if dailyStats[dateKey] == nil {
			dailyStats[dateKey] = map[string]int{
				"total":   0,
				"correct": 0,
			}
		}

		dailyStats[dateKey]["total"]++
		if pred.IsCorrect != nil && *pred.IsCorrect {
			dailyStats[dateKey]["correct"]++
		}
	}

	// 计算每日准确率
	dailyAccuracy := make(map[string]float64)
	for date, stats := range dailyStats {
		if stats["total"] > 0 {
			dailyAccuracy[date] = float64(stats["correct"]) / float64(stats["total"]) * 100
		}
	}

	return map[string]interface{}{
		"daily_stats":    dailyStats,
		"daily_accuracy": dailyAccuracy,
		"period_days":    days,
		"generated_at":   time.Now(),
	}, nil
}

// GetTrendAnalysis 获取趋势分析
func (sc *StatisticsCalculator) GetTrendAnalysis() (map[string]interface{}, error) {
	predictions, err := sc.mysql.GetLatestPredictions(50) // 分析最近50期
	if err != nil {
		return nil, fmt.Errorf("failed to get predictions for trend analysis: %v", err)
	}

	var recentAccuracy []bool
	var confidenceTrend []float64

	for _, pred := range predictions {
		if pred.IsCorrect != nil {
			recentAccuracy = append(recentAccuracy, *pred.IsCorrect)
		}
		if pred.ConfidenceScore != nil {
			confidenceTrend = append(confidenceTrend, *pred.ConfidenceScore)
		}
	}

	// 计算移动平均准确率
	movingAverage := sc.calculateMovingAverage(recentAccuracy, 10)

	return map[string]interface{}{
		"recent_accuracy":  recentAccuracy,
		"confidence_trend": confidenceTrend,
		"moving_average":   movingAverage,
		"trend_direction":  sc.analyzeTrendDirection(movingAverage),
		"analysis_time":    time.Now(),
	}, nil
}

// calculateMovingAverage 计算移动平均
func (sc *StatisticsCalculator) calculateMovingAverage(accuracy []bool, window int) []float64 {
	if len(accuracy) < window {
		return []float64{}
	}

	var movingAvg []float64
	for i := window - 1; i < len(accuracy); i++ {
		correct := 0
		for j := i - window + 1; j <= i; j++ {
			if accuracy[j] {
				correct++
			}
		}
		movingAvg = append(movingAvg, float64(correct)/float64(window)*100)
	}

	return movingAvg
}

// analyzeTrendDirection 分析趋势方向
func (sc *StatisticsCalculator) analyzeTrendDirection(movingAverage []float64) string {
	if len(movingAverage) < 2 {
		return "insufficient_data"
	}

	recent := movingAverage[len(movingAverage)-1]
	previous := movingAverage[len(movingAverage)-2]

	if recent > previous+1 {
		return "improving"
	} else if recent < previous-1 {
		return "declining"
	} else {
		return "stable"
	}
}
