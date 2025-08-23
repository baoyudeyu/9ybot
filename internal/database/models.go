package database

import (
	"time"
)

// LotteryResult 开奖数据模型
type LotteryResult struct {
	ID             int64     `json:"id" db:"id"`
	Qihao          string    `json:"qihao" db:"qihao"`
	OpenTime       time.Time `json:"opentime" db:"opentime"`
	OpenTimeString string    `json:"opentime_string" db:"opentime_string"` // API原始时间字符串
	OpenNum        string    `json:"opennum" db:"opennum"`
	SumValue       int       `json:"sum_value" db:"sum_value"`
	CreatedAt      time.Time `json:"created_at" db:"created_at"`
	UpdatedAt      time.Time `json:"updated_at" db:"updated_at"`
}

// Prediction 预测记录模型
type Prediction struct {
	ID               int64      `json:"id" db:"id"`
	TargetQihao      string     `json:"target_qihao" db:"target_qihao"`
	PredictedNum     string     `json:"predicted_num" db:"predicted_num"`
	PredictedSum     int        `json:"predicted_sum" db:"predicted_sum"`
	PredictedOddEven string     `json:"predicted_odd_even" db:"predicted_odd_even"` // 预测单双：单/双
	ActualNum        *string    `json:"actual_num" db:"actual_num"`
	ActualSum        *int       `json:"actual_sum" db:"actual_sum"`
	ActualOddEven    *string    `json:"actual_odd_even" db:"actual_odd_even"` // 实际单双：单/双
	IsCorrect        *bool      `json:"is_correct" db:"is_correct"`
	ConfidenceScore  *float64   `json:"confidence_score" db:"confidence_score"`
	AlgorithmVersion string     `json:"algorithm_version" db:"algorithm_version"`
	PredictedAt      time.Time  `json:"predicted_at" db:"predicted_at"`
	VerifiedAt       *time.Time `json:"verified_at" db:"verified_at"`
	CreatedAt        time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at" db:"updated_at"`
}

// CacheStatus 缓存状态模型
type CacheStatus struct {
	ID          int64     `json:"id" db:"id"`
	CacheKey    string    `json:"cache_key" db:"cache_key"`
	LastUpdated time.Time `json:"last_updated" db:"last_updated"`
	DataVersion string    `json:"data_version" db:"data_version"`
	Status      string    `json:"status" db:"status"`
}

// PredictionStats 预测统计模型
type PredictionStats struct {
	TotalPredictions   int       `json:"total_predictions" db:"total_predictions"`
	CorrectPredictions int       `json:"correct_predictions" db:"correct_predictions"`
	AccuracyRate       float64   `json:"accuracy_rate" db:"accuracy_rate"`
	FirstPrediction    time.Time `json:"first_prediction" db:"first_prediction"`
	LastPrediction     time.Time `json:"last_prediction" db:"last_prediction"`
}

// APIResponse API响应模型
type APIResponse struct {
	Data    []APILotteryData `json:"data"`
	Message string           `json:"message"`
}

// APILotteryData API返回的开奖数据模型
type APILotteryData struct {
	Qihao    string `json:"qihao"`
	OpenTime string `json:"opentime"`
	OpenNum  string `json:"opennum"`
	Sum      string `json:"sum"`
}

// PredictionRequest 预测请求模型
type PredictionRequest struct {
	HistoryData []LotteryResult `json:"history_data"`
	TargetQihao string          `json:"target_qihao"`
}

// PredictionResult 预测结果模型
type PredictionResult struct {
	TargetQihao      string    `json:"target_qihao"`
	PredictedNum     string    `json:"predicted_num"`
	ConfidenceScore  float64   `json:"confidence_score"`
	AlgorithmVersion string    `json:"algorithm_version"`
	Timestamp        time.Time `json:"timestamp"`
}

// TelegramMessage Telegram消息模型
type TelegramMessage struct {
	ChatID      int64       `json:"chat_id"`
	Text        string      `json:"text"`
	ParseMode   string      `json:"parse_mode"`
	ReplyMarkup interface{} `json:"reply_markup,omitempty"`
}

// CacheItem 缓存项模型
type CacheItem struct {
	Key       string        `json:"key"`
	Value     interface{}   `json:"value"`
	TTL       time.Duration `json:"ttl"`
	CreatedAt time.Time     `json:"created_at"`
	UpdatedAt time.Time     `json:"updated_at"`
}

// AppStatus 应用状态模型
type AppStatus struct {
	LastAPICall      time.Time `json:"last_api_call"`
	LastDataUpdate   time.Time `json:"last_data_update"`
	LastPrediction   time.Time `json:"last_prediction"`
	TotalPredictions int       `json:"total_predictions"`
	IsRunning        bool      `json:"is_running"`
	Version          string    `json:"version"`
}

// CalculateOddEven 计算单双
func CalculateOddEven(sum int) string {
	if sum%2 == 0 {
		return "双"
	}
	return "单"
}

// ParseOddEven 解析单双字符串
func ParseOddEven(oddEvenStr string) string {
	if oddEvenStr == "双" || oddEvenStr == "偶" || oddEvenStr == "even" {
		return "双"
	}
	return "单"
}
