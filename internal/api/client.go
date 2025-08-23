package api

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"
	"time"

	"pc28-bot/internal/config"
	"pc28-bot/internal/database"
	"pc28-bot/internal/logger"
)

// Client API客户端
type Client struct {
	httpClient *http.Client
	baseURL    string
	retryCount int
	retryDelay time.Duration
}

// NewClient 创建新的API客户端
func NewClient(cfg *config.API) *Client {
	return &Client{
		httpClient: &http.Client{
			Timeout: cfg.Timeout,
		},
		baseURL:    cfg.URL,
		retryCount: cfg.RetryCount,
		retryDelay: cfg.RetryDelay,
	}
}

// FetchLotteryData 获取开奖数据
func (c *Client) FetchLotteryData(limit int) (*database.APIResponse, error) {
	url := fmt.Sprintf("%s?limit=%d", c.baseURL, limit)

	var lastErr error
	for attempt := 0; attempt <= c.retryCount; attempt++ {
		if attempt > 0 {
			logger.Warnf("API request retry attempt %d/%d", attempt, c.retryCount)
			time.Sleep(c.retryDelay * time.Duration(attempt)) // 指数退避
		}

		resp, err := c.makeRequest(url)
		if err != nil {
			lastErr = err
			continue
		}

		return resp, nil
	}

	return nil, fmt.Errorf("failed to fetch lottery data after %d attempts: %v", c.retryCount, lastErr)
}

// makeRequest 执行HTTP请求
func (c *Client) makeRequest(url string) (*database.APIResponse, error) {
	logger.Debugf("Making API request to: %s", url)

	resp, err := c.httpClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP request failed with status: %d", resp.StatusCode)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %v", err)
	}

	var apiResponse database.APIResponse
	if err := json.Unmarshal(body, &apiResponse); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %v", err)
	}

	if apiResponse.Message != "success" {
		return nil, fmt.Errorf("API returned error message: %s", apiResponse.Message)
	}

	// 只在调试模式下记录详细信息
	if len(apiResponse.Data) > 0 {
		logger.Debugf("API request successful, got %d records", len(apiResponse.Data))
	}
	return &apiResponse, nil
}

// ConvertAPIDataToLotteryResult 转换API数据为内部数据模型
func (c *Client) ConvertAPIDataToLotteryResult(apiData database.APILotteryData) (*database.LotteryResult, error) {
	// 解析开奖时间
	openTime, err := c.parseOpenTime(apiData.OpenTime)
	if err != nil {
		return nil, fmt.Errorf("failed to parse open time: %v", err)
	}

	// 解析和值
	sumValue, err := strconv.Atoi(apiData.Sum)
	if err != nil {
		return nil, fmt.Errorf("failed to parse sum value: %v", err)
	}

	// 验证开奖号码格式
	if err := c.validateOpenNum(apiData.OpenNum); err != nil {
		return nil, fmt.Errorf("invalid open number format: %v", err)
	}

	return &database.LotteryResult{
		Qihao:          apiData.Qihao,
		OpenTime:       openTime,
		OpenTimeString: apiData.OpenTime, // 保存API原始时间字符串
		OpenNum:        apiData.OpenNum,
		SumValue:       sumValue,
	}, nil
}

// parseOpenTime 解析开奖时间
func (c *Client) parseOpenTime(timeStr string) (time.Time, error) {
	// API返回格式: "08-23 01:16:00"
	// 需要补充年份
	currentYear := time.Now().Year()
	fullTimeStr := fmt.Sprintf("%d-%s", currentYear, timeStr)

	// 尝试解析时间
	layouts := []string{
		"2006-01-02 15:04:05",
		"2006-1-2 15:04:05",
		"2006-01-2 15:04:05",
		"2006-1-02 15:04:05",
	}

	for _, layout := range layouts {
		if t, err := time.Parse(layout, fullTimeStr); err == nil {
			return t, nil
		}
	}

	return time.Time{}, fmt.Errorf("unable to parse time: %s", timeStr)
}

// validateOpenNum 验证开奖号码格式
func (c *Client) validateOpenNum(openNum string) error {
	parts := strings.Split(openNum, "+")
	if len(parts) != 3 {
		return fmt.Errorf("open number should have 3 parts, got %d", len(parts))
	}

	for i, part := range parts {
		num, err := strconv.Atoi(strings.TrimSpace(part))
		if err != nil {
			return fmt.Errorf("invalid number in part %d: %s", i+1, part)
		}
		if num < 0 || num > 9 {
			return fmt.Errorf("number out of range (0-9) in part %d: %d", i+1, num)
		}
	}

	return nil
}

// FetchAndValidateLatestData 获取并验证最新数据
func (c *Client) FetchAndValidateLatestData() (*database.LotteryResult, error) {
	// 获取最新的10条数据
	apiResponse, err := c.FetchLotteryData(10)
	if err != nil {
		return nil, err
	}

	if len(apiResponse.Data) == 0 {
		return nil, fmt.Errorf("no data returned from API")
	}

	// 获取最新的一条数据
	latestAPIData := apiResponse.Data[0]

	// 转换为内部数据模型
	lotteryResult, err := c.ConvertAPIDataToLotteryResult(latestAPIData)
	if err != nil {
		return nil, err
	}

	// 额外验证：检查和值是否正确
	nums, err := database.ParseOpenNum(lotteryResult.OpenNum)
	if err != nil {
		return nil, fmt.Errorf("failed to parse open numbers for validation: %v", err)
	}

	calculatedSum := database.CalculateSum(nums)
	if calculatedSum != lotteryResult.SumValue {
		logger.Warnf("Sum value mismatch: calculated=%d, received=%d", calculatedSum, lotteryResult.SumValue)
		// 使用计算出的和值
		lotteryResult.SumValue = calculatedSum
	}

	logger.Debugf("Latest lottery data validated: %s, %s, sum=%d",
		lotteryResult.Qihao, lotteryResult.OpenNum, lotteryResult.SumValue)

	return lotteryResult, nil
}

// CheckDataFreshness 检查数据新鲜度
func (c *Client) CheckDataFreshness(latestTime time.Time) bool {
	// PC28每3.5分钟开奖一次
	expectedInterval := 3*time.Minute + 30*time.Second
	timeSinceLatest := time.Since(latestTime)

	// 如果距离最后开奖时间超过5分钟，认为数据可能不新鲜
	threshold := 5 * time.Minute

	if timeSinceLatest > threshold {
		logger.Warnf("Data may not be fresh: last update %v ago", timeSinceLatest)
		return false
	}

	// 检查是否应该有新数据
	if timeSinceLatest > expectedInterval {
		logger.Infof("Expected new data: last update %v ago", timeSinceLatest)
		return false
	}

	return true
}

// GetHistoricalData 获取历史数据
func (c *Client) GetHistoricalData(limit int) ([]database.LotteryResult, error) {
	apiResponse, err := c.FetchLotteryData(limit)
	if err != nil {
		return nil, err
	}

	var results []database.LotteryResult
	for _, apiData := range apiResponse.Data {
		result, err := c.ConvertAPIDataToLotteryResult(apiData)
		if err != nil {
			logger.Warnf("Failed to convert API data: %v", err)
			continue
		}
		results = append(results, *result)
	}

	if len(results) == 0 {
		return nil, fmt.Errorf("no valid data could be converted")
	}

	logger.Infof("Retrieved %d historical lottery results", len(results))
	return results, nil
}

// HealthCheck 检查API健康状态
func (c *Client) HealthCheck() error {
	_, err := c.FetchLotteryData(1)
	if err != nil {
		return fmt.Errorf("API health check failed: %v", err)
	}

	logger.Debug("API health check passed")
	return nil
}

// GetAPIStats 获取API统计信息
func (c *Client) GetAPIStats() map[string]interface{} {
	return map[string]interface{}{
		"base_url":    c.baseURL,
		"timeout":     c.httpClient.Timeout,
		"retry_count": c.retryCount,
		"retry_delay": c.retryDelay,
	}
}
