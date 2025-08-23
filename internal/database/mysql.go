package database

import (
	"database/sql"
	"fmt"
	"strconv"
	"strings"

	"pc28-bot/internal/config"
	"pc28-bot/internal/logger"

	_ "github.com/go-sql-driver/mysql"
)

// MySQLDB MySQL数据库客户端
type MySQLDB struct {
	db *sql.DB
}

// NewMySQLDB 创建新的MySQL数据库连接
func NewMySQLDB(cfg *config.Database) (*MySQLDB, error) {
	db, err := sql.Open("mysql", cfg.GetDSN())
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %v", err)
	}

	// 设置连接池参数
	db.SetMaxOpenConns(cfg.MaxOpenConns)
	db.SetMaxIdleConns(cfg.MaxIdleConns)
	db.SetConnMaxLifetime(cfg.ConnMaxLifetime)

	// 测试连接
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %v", err)
	}

	mysqlDB := &MySQLDB{db: db}

	// 自动创建表结构
	if err := mysqlDB.createTablesIfNotExists(); err != nil {
		return nil, fmt.Errorf("failed to create tables: %v", err)
	}

	return mysqlDB, nil
}

// Close 关闭数据库连接
func (m *MySQLDB) Close() error {
	return m.db.Close()
}

// SaveLotteryResult 保存开奖数据
func (m *MySQLDB) SaveLotteryResult(result *LotteryResult) error {
	query := `INSERT INTO lottery_results (qihao, opentime, opentime_string, opennum, sum_value) 
			  VALUES (?, ?, ?, ?, ?) 
			  ON DUPLICATE KEY UPDATE 
			  opentime = VALUES(opentime), 
			  opentime_string = VALUES(opentime_string),
			  opennum = VALUES(opennum), 
			  sum_value = VALUES(sum_value),
			  updated_at = CURRENT_TIMESTAMP`

	_, err := m.db.Exec(query, result.Qihao, result.OpenTime, result.OpenTimeString, result.OpenNum, result.SumValue)
	if err != nil {
		return fmt.Errorf("failed to save lottery result: %v", err)
	}

	logger.Debugf("Saved lottery result: %s", result.Qihao)
	return nil
}

// GetLatestLotteryResults 获取最新的开奖数据
func (m *MySQLDB) GetLatestLotteryResults(limit int) ([]LotteryResult, error) {
	query := `SELECT id, qihao, opentime, opentime_string, opennum, sum_value, created_at, updated_at 
			  FROM lottery_results 
			  ORDER BY opentime DESC 
			  LIMIT ?`

	rows, err := m.db.Query(query, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query latest lottery results: %v", err)
	}
	defer rows.Close()

	var results []LotteryResult
	for rows.Next() {
		var result LotteryResult
		err := rows.Scan(&result.ID, &result.Qihao, &result.OpenTime, &result.OpenTimeString,
			&result.OpenNum, &result.SumValue, &result.CreatedAt, &result.UpdatedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan lottery result: %v", err)
		}
		results = append(results, result)
	}

	return results, nil
}

// GetLotteryResultByQihao 根据期号获取开奖数据
func (m *MySQLDB) GetLotteryResultByQihao(qihao string) (*LotteryResult, error) {
	query := `SELECT id, qihao, opentime, opentime_string, opennum, sum_value, created_at, updated_at 
			  FROM lottery_results 
			  WHERE qihao = ?`

	var result LotteryResult
	err := m.db.QueryRow(query, qihao).Scan(
		&result.ID, &result.Qihao, &result.OpenTime, &result.OpenTimeString,
		&result.OpenNum, &result.SumValue, &result.CreatedAt, &result.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get lottery result by qihao: %v", err)
	}

	return &result, nil
}

// SavePrediction 保存预测记录
func (m *MySQLDB) SavePrediction(prediction *Prediction) error {
	// 计算预测和值
	predictedSum := prediction.PredictedSum
	if predictedSum == 0 {
		// 如果没有提供，尝试从预测号码计算
		if nums, err := ParseOpenNum(prediction.PredictedNum); err == nil {
			predictedSum = CalculateSum(nums)
		}
	}

	// 计算预测单双
	predictedOddEven := prediction.PredictedOddEven
	if predictedOddEven == "" {
		predictedOddEven = CalculateOddEven(predictedSum)
	}

	query := `INSERT INTO predictions (target_qihao, predicted_num, predicted_sum, predicted_odd_even, confidence_score, algorithm_version, predicted_at) 
			  VALUES (?, ?, ?, ?, ?, ?, ?)`

	result, err := m.db.Exec(query, prediction.TargetQihao, prediction.PredictedNum, predictedSum, predictedOddEven,
		prediction.ConfidenceScore, prediction.AlgorithmVersion, prediction.PredictedAt)
	if err != nil {
		return fmt.Errorf("failed to save prediction: %v", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("failed to get last insert id: %v", err)
	}

	prediction.ID = id
	prediction.PredictedSum = predictedSum
	logger.Debugf("Saved prediction for qihao: %s", prediction.TargetQihao)
	return nil
}

// UpdatePredictionResult 更新预测结果
func (m *MySQLDB) UpdatePredictionResult(qihao string, actualNum string, isCorrect bool) error {
	query := `UPDATE predictions 
			  SET actual_num = ?, is_correct = ?, verified_at = NOW() 
			  WHERE target_qihao = ?`

	_, err := m.db.Exec(query, actualNum, isCorrect, qihao)
	if err != nil {
		return fmt.Errorf("failed to update prediction result: %v", err)
	}

	logger.Debugf("Updated prediction result for qihao: %s, correct: %t", qihao, isCorrect)
	return nil
}

// GetLatestPredictions 获取最新的预测记录
func (m *MySQLDB) GetLatestPredictions(limit int) ([]Prediction, error) {
	query := `SELECT id, target_qihao, predicted_num, predicted_sum, predicted_odd_even, 
			  actual_num, actual_sum, actual_odd_even, is_correct, 
			  confidence_score, algorithm_version, predicted_at, verified_at,
			  created_at, updated_at
			  FROM predictions 
			  ORDER BY CAST(target_qihao AS UNSIGNED) DESC 
			  LIMIT ?`

	rows, err := m.db.Query(query, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query latest predictions: %v", err)
	}
	defer rows.Close()

	var predictions []Prediction
	for rows.Next() {
		var prediction Prediction
		err := rows.Scan(&prediction.ID, &prediction.TargetQihao, &prediction.PredictedNum,
			&prediction.PredictedSum, &prediction.PredictedOddEven,
			&prediction.ActualNum, &prediction.ActualSum, &prediction.ActualOddEven,
			&prediction.IsCorrect, &prediction.ConfidenceScore,
			&prediction.AlgorithmVersion, &prediction.PredictedAt, &prediction.VerifiedAt,
			&prediction.CreatedAt, &prediction.UpdatedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan prediction: %v", err)
		}
		predictions = append(predictions, prediction)
	}

	return predictions, nil
}

// GetPredictionStats 获取预测统计信息
func (m *MySQLDB) GetPredictionStats() (*PredictionStats, error) {
	query := `SELECT 
		COUNT(*) as total_predictions,
		SUM(CASE WHEN is_correct = 1 THEN 1 ELSE 0 END) as correct_predictions,
		ROUND(
			(SUM(CASE WHEN is_correct = 1 THEN 1 ELSE 0 END) * 100.0 / COUNT(*)), 2
		) as accuracy_rate,
		MIN(predicted_at) as first_prediction,
		MAX(predicted_at) as last_prediction
	FROM predictions 
	WHERE is_correct IS NOT NULL`

	var stats PredictionStats
	err := m.db.QueryRow(query).Scan(
		&stats.TotalPredictions, &stats.CorrectPredictions,
		&stats.AccuracyRate, &stats.FirstPrediction, &stats.LastPrediction,
	)

	if err == sql.ErrNoRows {
		return &PredictionStats{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get prediction stats: %v", err)
	}

	return &stats, nil
}

// GetLotteryHistory 获取历史开奖数据
func (m *MySQLDB) GetLotteryHistory(limit int) ([]LotteryResult, error) {
	query := `SELECT id, qihao, opentime, opentime_string, opennum, sum_value, created_at, updated_at 
			   FROM lottery_results 
			   ORDER BY qihao DESC 
			   LIMIT ?`

	rows, err := m.db.Query(query, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query lottery history: %v", err)
	}
	defer rows.Close()

	var results []LotteryResult
	for rows.Next() {
		var result LotteryResult
		err := rows.Scan(
			&result.ID,
			&result.Qihao,
			&result.OpenTime,
			&result.OpenTimeString,
			&result.OpenNum,
			&result.SumValue,
			&result.CreatedAt,
			&result.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan lottery result: %v", err)
		}
		results = append(results, result)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error reading lottery history rows: %v", err)
	}

	return results, nil
}

// ValidatePrediction 验证预测结果（简化版）
func (m *MySQLDB) ValidatePrediction(qihao string, actualResult *LotteryResult) (bool, error) {
	// 获取对应的预测记录
	query := `SELECT predicted_num, predicted_sum, predicted_odd_even FROM predictions WHERE target_qihao = ? ORDER BY predicted_at DESC LIMIT 1`

	var predictedNum string
	var predictedSum int
	var predictedOddEven string
	err := m.db.QueryRow(query, qihao).Scan(&predictedNum, &predictedSum, &predictedOddEven)
	if err == sql.ErrNoRows {
		return false, fmt.Errorf("no prediction found for qihao: %s", qihao)
	}
	if err != nil {
		return false, fmt.Errorf("failed to get prediction: %v", err)
	}

	// 计算实际单双
	actualOddEven := CalculateOddEven(actualResult.SumValue)

	// 单双预测验证（主要验证方式）
	isCorrect := predictedOddEven == actualOddEven

	// 更新预测结果，包含实际和值和单双
	updateQuery := `UPDATE predictions 
					SET actual_num = ?, actual_sum = ?, actual_odd_even = ?, is_correct = ?, verified_at = NOW() 
					WHERE target_qihao = ?`

	_, err = m.db.Exec(updateQuery, actualResult.OpenNum, actualResult.SumValue, actualOddEven, isCorrect, qihao)
	if err != nil {
		return false, fmt.Errorf("failed to update prediction result: %v", err)
	}

	return isCorrect, nil
}

// createTablesIfNotExists 自动创建表结构
func (m *MySQLDB) createTablesIfNotExists() error {
	// 首先检查是否已存在表
	var tableCount int
	err := m.db.QueryRow("SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = 'lottery_results'").Scan(&tableCount)
	if err != nil {
		return fmt.Errorf("failed to check table existence: %v", err)
	}

	if tableCount == 0 {
		// 创建开奖数据表
		createLotteryResultsTable := `CREATE TABLE lottery_results (
			id BIGINT AUTO_INCREMENT PRIMARY KEY,
			qihao VARCHAR(20) UNIQUE NOT NULL COMMENT '期号',
			opentime DATETIME NOT NULL COMMENT '开奖时间',
			opentime_string VARCHAR(50) NOT NULL COMMENT 'API原始时间字符串',
			opennum VARCHAR(20) NOT NULL COMMENT '开奖号码',
			sum_value INT NOT NULL COMMENT '和值',
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP COMMENT '记录创建时间',
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '记录更新时间',
			INDEX idx_qihao (qihao),
			INDEX idx_opentime (opentime),
			INDEX idx_created_at (created_at)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='开奖数据表'`

		if _, err := m.db.Exec(createLotteryResultsTable); err != nil {
			return fmt.Errorf("failed to create lottery_results table: %v", err)
		}
	}

	// 检查预测表
	err = m.db.QueryRow("SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = 'predictions'").Scan(&tableCount)
	if err != nil {
		return fmt.Errorf("failed to check predictions table existence: %v", err)
	}

	if tableCount == 0 {
		// 创建预测记录表
		createPredictionsTable := `CREATE TABLE predictions (
			id BIGINT AUTO_INCREMENT PRIMARY KEY,
			target_qihao VARCHAR(20) NOT NULL COMMENT '目标期号',
			predicted_num VARCHAR(20) NOT NULL COMMENT '预测号码',
			actual_num VARCHAR(20) DEFAULT NULL COMMENT '实际开奖号码',
			is_correct BOOLEAN DEFAULT NULL COMMENT '是否预测正确',
			confidence_score DECIMAL(5,2) DEFAULT NULL COMMENT '置信度评分',
			algorithm_version VARCHAR(50) DEFAULT 'default' COMMENT '算法版本',
			predicted_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP COMMENT '预测时间',
			verified_at TIMESTAMP NULL COMMENT '验证时间',
			INDEX idx_target_qihao (target_qihao),
			INDEX idx_predicted_at (predicted_at),
			INDEX idx_is_correct (is_correct),
			INDEX idx_verified_at (verified_at)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='预测记录表'`

		if _, err := m.db.Exec(createPredictionsTable); err != nil {
			return fmt.Errorf("failed to create predictions table: %v", err)
		}
	}

	return nil
}

// CleanOldData 清理旧数据
func (m *MySQLDB) CleanOldData() error {
	// 清理超过24小时的开奖数据
	_, err := m.db.Exec("DELETE FROM lottery_results WHERE created_at < DATE_SUB(NOW(), INTERVAL 24 HOUR)")
	if err != nil {
		return fmt.Errorf("failed to clean lottery results: %v", err)
	}

	// 清理超过24小时的预测记录
	_, err = m.db.Exec("DELETE FROM predictions WHERE predicted_at < DATE_SUB(NOW(), INTERVAL 24 HOUR)")
	if err != nil {
		return fmt.Errorf("failed to clean predictions: %v", err)
	}

	return nil
}

// CheckNewQihao 检查是否有新的期号
func (m *MySQLDB) CheckNewQihao(qihao string) (bool, error) {
	// 先测试表是否存在
	var tableExists int
	err := m.db.QueryRow("SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = 'lottery_results'").Scan(&tableExists)
	if err != nil {
		return false, fmt.Errorf("failed to check table existence: %v", err)
	}

	if tableExists == 0 {
		return true, nil // 表不存在，认为是新数据
	}

	var count int
	query := "SELECT COUNT(*) FROM lottery_results WHERE qihao = ?"
	err = m.db.QueryRow(query, qihao).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("failed to check new qihao: %v", err)
	}

	return count == 0, nil
}

// GetUnverifiedPredictions 获取所有未验证的预测记录
func (m *MySQLDB) GetUnverifiedPredictions() ([]Prediction, error) {
	query := `SELECT id, target_qihao, predicted_num, predicted_sum, predicted_odd_even, 
			  actual_num, actual_sum, actual_odd_even, is_correct, 
			  confidence_score, algorithm_version, predicted_at, verified_at,
			  created_at, updated_at
			  FROM predictions 
			  WHERE is_correct IS NULL AND actual_num IS NULL
			  ORDER BY predicted_at DESC`

	rows, err := m.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query unverified predictions: %v", err)
	}
	defer rows.Close()

	var predictions []Prediction
	for rows.Next() {
		var prediction Prediction
		err := rows.Scan(&prediction.ID, &prediction.TargetQihao, &prediction.PredictedNum,
			&prediction.PredictedSum, &prediction.PredictedOddEven,
			&prediction.ActualNum, &prediction.ActualSum, &prediction.ActualOddEven,
			&prediction.IsCorrect, &prediction.ConfidenceScore,
			&prediction.AlgorithmVersion, &prediction.PredictedAt, &prediction.VerifiedAt,
			&prediction.CreatedAt, &prediction.UpdatedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan unverified prediction: %v", err)
		}
		predictions = append(predictions, prediction)
	}

	return predictions, nil
}

// CleanupExpiredPredictions 清理过期的待开奖预测记录
func (m *MySQLDB) CleanupExpiredPredictions(latestQihao string) (int, error) {
	// 删除目标期号小于最新期号且仍未验证的预测记录
	query := `DELETE FROM predictions 
			  WHERE target_qihao < ? AND is_correct IS NULL AND actual_num IS NULL`

	result, err := m.db.Exec(query, latestQihao)
	if err != nil {
		return 0, fmt.Errorf("failed to cleanup expired predictions: %v", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("failed to get rows affected: %v", err)
	}

	return int(rowsAffected), nil
}

// GetNextQihao 获取下一个期号
func (m *MySQLDB) GetNextQihao() (string, error) {
	query := `SELECT qihao FROM lottery_results ORDER BY opentime DESC LIMIT 1`

	var latestQihao string
	err := m.db.QueryRow(query).Scan(&latestQihao)
	if err == sql.ErrNoRows {
		return "3326001", nil // 默认起始期号
	}
	if err != nil {
		return "", fmt.Errorf("failed to get latest qihao: %v", err)
	}

	// 解析期号并增加1
	if len(latestQihao) >= 7 {
		prefix := latestQihao[:4]
		numStr := latestQihao[4:]
		num, err := strconv.Atoi(numStr)
		if err != nil {
			return "", fmt.Errorf("failed to parse qihao number: %v", err)
		}
		return fmt.Sprintf("%s%03d", prefix, num+1), nil
	}

	return "", fmt.Errorf("invalid qihao format: %s", latestQihao)
}

// ParseOpenNum 解析开奖号码字符串
func ParseOpenNum(openNum string) ([]int, error) {
	parts := strings.Split(openNum, "+")
	if len(parts) != 3 {
		return nil, fmt.Errorf("invalid opennum format: %s", openNum)
	}

	var nums []int
	for _, part := range parts {
		num, err := strconv.Atoi(strings.TrimSpace(part))
		if err != nil {
			return nil, fmt.Errorf("failed to parse number: %s", part)
		}
		nums = append(nums, num)
	}

	return nums, nil
}

// FormatOpenNum 格式化开奖号码
func FormatOpenNum(nums []int) string {
	if len(nums) != 3 {
		return ""
	}
	return fmt.Sprintf("%d+%d+%d", nums[0], nums[1], nums[2])
}

// CalculateSum 计算和值
func CalculateSum(nums []int) int {
	sum := 0
	for _, num := range nums {
		sum += num
	}
	return sum
}
