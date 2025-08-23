package main

import (
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"pc28-bot/internal/api"
	"pc28-bot/internal/cache"
	"pc28-bot/internal/config"
	"pc28-bot/internal/database"
	"pc28-bot/internal/logger"
	"pc28-bot/internal/predictor"
	"pc28-bot/internal/telegram"
)

// App 应用程序主结构
type App struct {
	config         *config.Config
	mysql          *database.MySQLDB
	cacheManager   *cache.CacheManager
	apiClient      *api.Client
	predictorMgr   *predictor.PredictorManager
	validator      *predictor.Validator
	statCalculator *predictor.StatisticsCalculator
	telegramBot    *telegram.Bot

	// 控制通道
	stopChannel chan bool
	wg          sync.WaitGroup

	// 错误状态跟踪（避免重复日志）
	lastAPIError       string
	lastDBError        string
	lastProcessedQihao string
}

// NewApp 创建应用程序实例
func NewApp(configPath string) (*App, error) {
	// 加载配置
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %v", err)
	}

	// 初始化日志
	logger.InitLogger(cfg.App.LogLevel)
	fmt.Println("🚀 启动PC28预测机器人...")

	// 初始化数据库
	mysql, err := database.NewMySQLDB(&cfg.Database)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize database: %v", err)
	}
	fmt.Println("✅ 数据库连接成功")
	fmt.Println("✅ 数据库表结构初始化完成")

	// 初始化缓存管理器
	cacheManager, err := cache.NewCacheManager(mysql, cfg.App.CacheTTL)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize cache manager: %v", err)
	}
	fmt.Println("✅ 缓存系统初始化完成")

	// 初始化API客户端
	apiClient := api.NewClient(&cfg.API)

	// 初始化预测器管理器
	predictorMgr := predictor.NewPredictorManager()

	// 初始化验证器和统计计算器
	validator := predictor.NewValidator(mysql)
	statCalculator := predictor.NewStatisticsCalculator(mysql)

	// 初始化Telegram机器人
	telegramBot, err := telegram.NewBot(&cfg.Telegram, cacheManager)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize telegram bot: %v", err)
	}
	fmt.Println("✅ Telegram机器人连接成功")

	app := &App{
		config:         cfg,
		mysql:          mysql,
		cacheManager:   cacheManager,
		apiClient:      apiClient,
		predictorMgr:   predictorMgr,
		validator:      validator,
		statCalculator: statCalculator,
		telegramBot:    telegramBot,
		stopChannel:    make(chan bool),
	}

	fmt.Println("🎯 应用程序初始化完成")
	return app, nil
}

// Start 启动应用程序
func (a *App) Start() error {
	fmt.Println("🔄 启动所有服务...")

	// 初始化历史数据
	if err := a.initializeHistoricalData(); err != nil {
		logger.Warnf("Failed to initialize historical data: %v", err)
	}

	// 启动Telegram机器人
	a.telegramBot.Start()

	// 启动数据监控协程
	a.wg.Add(1)
	go a.dataMonitorLoop()

	// 启动数据清理协程
	a.wg.Add(1)
	go a.dataCleanupLoop()

	fmt.Println("✅ 所有服务启动完成")
	fmt.Println("📡 开始监控PC28开奖数据...")
	fmt.Printf("⏰ 轮询间隔: %v\n", a.config.App.PollingInterval)
	fmt.Println("🔔 机器人仅在私聊中提供服务")
	fmt.Println("💡 按 Ctrl+C 停止程序")
	fmt.Println("")
	return nil
}

// Stop 停止应用程序
func (a *App) Stop() error {
	fmt.Println("🛑 正在停止应用程序...")

	// 发送停止信号
	close(a.stopChannel)

	// 停止Telegram机器人
	a.telegramBot.Stop()

	// 等待所有协程结束
	a.wg.Wait()

	// 关闭缓存管理器
	if err := a.cacheManager.Close(); err != nil {
		logger.Errorf("Failed to close cache manager: %v", err)
	}

	// 关闭数据库连接
	if err := a.mysql.Close(); err != nil {
		logger.Errorf("Failed to close database: %v", err)
	}

	fmt.Println("✅ 应用程序已安全停止")
	return nil
}

// initializeHistoricalData 初始化历史数据并同步预测验证
func (a *App) initializeHistoricalData() error {
	fmt.Println("📚 初始化历史开奖数据...")

	// 获取更多的API历史数据以确保覆盖所有未验证的预测
	historicalData, err := a.apiClient.GetHistoricalData(50) // 增加到50期
	if err != nil {
		return fmt.Errorf("failed to get historical data: %v", err)
	}

	// 保存到数据库（如果不存在的话）
	savedCount := 0
	for _, data := range historicalData {
		// 检查是否已存在
		existing, err := a.mysql.GetLotteryResultByQihao(data.Qihao)
		if err != nil || existing == nil {
			// 不存在，保存到数据库
			if err := a.mysql.SaveLotteryResult(&data); err != nil {
				logger.Warnf("Failed to save historical data %s: %v", data.Qihao, err)
				continue
			}
			savedCount++
		}
	}

	if savedCount > 0 {
		fmt.Printf("✅ 初始化了 %d 条历史数据\n", savedCount)
	} else {
		fmt.Println("✅ 历史数据已存在，无需初始化")
	}

	// 同步预测验证状态
	fmt.Println("🔍 检查并更新预测验证状态...")
	verifiedCount, err := a.syncPredictionVerifications(historicalData)
	if err != nil {
		logger.Warnf("Failed to sync prediction verifications: %v", err)
	} else if verifiedCount > 0 {
		fmt.Printf("✅ 更新了 %d 条预测验证结果\n", verifiedCount)
	}

	// 清理过期的待开奖预测
	fmt.Println("🧹 清理过期的待开奖预测...")
	cleanedCount, err := a.cleanupExpiredPredictions(historicalData)
	if err != nil {
		logger.Warnf("Failed to cleanup expired predictions: %v", err)
	} else if cleanedCount > 0 {
		fmt.Printf("✅ 清理了 %d 条过期预测\n", cleanedCount)
	}

	// 更新缓存
	if len(historicalData) > 0 {
		if err := a.cacheManager.OnNewLotteryData(&historicalData[0]); err != nil {
			logger.Warnf("Failed to update cache for historical data: %v", err)
		}
	}

	// 检查是否需要生成最新预测
	fmt.Println("🔍 检查是否需要生成最新预测...")
	if err := a.ensureLatestPrediction(); err != nil {
		logger.Warnf("Failed to ensure latest prediction: %v", err)
	}

	return nil
}

// syncPredictionVerifications 同步预测验证状态
func (a *App) syncPredictionVerifications(historicalData []database.LotteryResult) (int, error) {
	// 获取所有未验证的预测记录
	unverifiedPredictions, err := a.mysql.GetUnverifiedPredictions()
	if err != nil {
		return 0, fmt.Errorf("failed to get unverified predictions: %v", err)
	}

	if len(unverifiedPredictions) == 0 {
		return 0, nil
	}

	// 创建开奖数据的快速查找映射
	lotteryMap := make(map[string]*database.LotteryResult)
	for i := range historicalData {
		lotteryMap[historicalData[i].Qihao] = &historicalData[i]
	}

	verifiedCount := 0
	for _, prediction := range unverifiedPredictions {
		// 查找对应的开奖结果
		if lotteryResult, exists := lotteryMap[prediction.TargetQihao]; exists {
			// 验证预测结果
			_, err := a.validator.ValidatePrediction(prediction.TargetQihao, lotteryResult)
			if err != nil {
				logger.Warnf("Failed to validate prediction for %s: %v", prediction.TargetQihao, err)
				continue
			}
			verifiedCount++
			logger.Debugf("Verified prediction for %s", prediction.TargetQihao)
		}
	}

	return verifiedCount, nil
}

// cleanupExpiredPredictions 清理过期的待开奖预测
func (a *App) cleanupExpiredPredictions(historicalData []database.LotteryResult) (int, error) {
	// 获取最新的期号
	if len(historicalData) == 0 {
		return 0, nil
	}

	latestQihao := historicalData[0].Qihao

	// 删除目标期号小于最新期号且仍未验证的预测记录
	cleanedCount, err := a.mysql.CleanupExpiredPredictions(latestQihao)
	if err != nil {
		return 0, fmt.Errorf("failed to cleanup expired predictions: %v", err)
	}

	return cleanedCount, nil
}

// ensureLatestPrediction 确保有最新的预测
func (a *App) ensureLatestPrediction() error {
	// 获取最新的开奖数据
	latestLottery, err := a.cacheManager.GetLatestLotteryData()
	if err != nil {
		return fmt.Errorf("failed to get latest lottery data: %v", err)
	}

	// 获取最新的预测
	latestPrediction, err := a.cacheManager.GetLatestPrediction()
	if err != nil {
		// 没有预测记录，生成一个
		logger.Info("No prediction found, generating new prediction")
		return a.generateNewPrediction()
	}

	// 检查预测的目标期号是否是下一期
	expectedNextQihao := a.generateNextQihao(latestLottery.Qihao)
	if latestPrediction.TargetQihao != expectedNextQihao {
		// 预测的期号不是下一期，生成新预测
		logger.Infof("Prediction target %s != expected %s, generating new prediction",
			latestPrediction.TargetQihao, expectedNextQihao)
		return a.generateNewPrediction()
	}

	logger.Info("Latest prediction is up to date")
	return nil
}

// generateNextQihao 生成下一期期号（辅助方法）
func (a *App) generateNextQihao(latestQihao string) string {
	// 尝试直接解析整个期号为数字
	var qihaoNum int
	if _, err := fmt.Sscanf(latestQihao, "%d", &qihaoNum); err == nil {
		return fmt.Sprintf("%d", qihaoNum+1)
	}

	// 如果解析失败，返回默认值
	logger.Warnf("Failed to parse qihao: %s, using default", latestQihao)
	return "3326999"
}

// dataMonitorLoop 数据监控循环
func (a *App) dataMonitorLoop() {
	defer a.wg.Done()

	ticker := time.NewTicker(a.config.App.PollingInterval)
	defer ticker.Stop()

	consecutiveErrors := 0
	lastNewData := time.Time{}

	for {
		select {
		case <-ticker.C:
			if err := a.processDataUpdate(); err != nil {
				consecutiveErrors++
				// 只在第一次错误和每30次错误时显示（减少刷屏）
				if consecutiveErrors == 1 {
					fmt.Printf("⚠️  数据获取失败: %v\n", err)
				} else if consecutiveErrors%30 == 0 {
					fmt.Printf("❌ 连续失败 %d 次，仍在重试...\n", consecutiveErrors)
				}
			} else {
				if consecutiveErrors > 0 {
					fmt.Printf("✅ 数据连接已恢复（失败了 %d 次）\n", consecutiveErrors)
					consecutiveErrors = 0
				}
				// 检查是否有新数据处理
				if time.Since(lastNewData) > 5*time.Minute {
					lastNewData = time.Now()
				}
			}
		case <-a.stopChannel:
			return
		}
	}
}

// processDataUpdate 处理数据更新
func (a *App) processDataUpdate() error {
	// 获取最新数据
	latestData, err := a.apiClient.FetchAndValidateLatestData()
	if err != nil {
		// 只在首次出错或错误类型变化时记录
		if a.lastAPIError != err.Error() {
			logger.Errorf("API fetch failed: %v", err)
			a.lastAPIError = err.Error()
		}
		return fmt.Errorf("failed to fetch latest data: %v", err)
	}
	a.lastAPIError = "" // 清除错误状态

	// 检查是否是新数据
	isNew, err := a.mysql.CheckNewQihao(latestData.Qihao)
	if err != nil {
		// 只在首次出错或错误类型变化时记录
		if a.lastDBError != err.Error() {
			logger.Errorf("Database check failed: %v", err)
			a.lastDBError = err.Error()
		}
		return fmt.Errorf("failed to check new qihao: %v", err)
	}
	a.lastDBError = "" // 清除错误状态

	if !isNew {
		// 不是新数据，跳过处理（不记录日志避免重复）
		return nil
	}

	fmt.Printf("🎯 发现新开奖: %s - %s (和值:%d)\n", latestData.Qihao, latestData.OpenNum, latestData.SumValue)

	// 保存新数据到数据库
	if err := a.mysql.SaveLotteryResult(latestData); err != nil {
		return fmt.Errorf("failed to save lottery result: %v", err)
	}

	// 更新缓存
	if err := a.cacheManager.OnNewLotteryData(latestData); err != nil {
		logger.Warnf("Failed to update cache for new data: %v", err)
	}

	// 验证之前的预测
	if err := a.verifyPreviousPrediction(latestData); err != nil {
		logger.Warnf("Failed to verify previous prediction: %v", err)
	}

	// 生成新预测
	if err := a.generateNewPrediction(); err != nil {
		logger.Errorf("Failed to generate new prediction: %v", err)
		return err
	}

	fmt.Printf("✅ 新数据处理完成: %s\n", latestData.Qihao)
	return nil
}

// verifyPreviousPrediction 验证之前的预测
func (a *App) verifyPreviousPrediction(actualResult *database.LotteryResult) error {
	// 验证预测结果
	validation, err := a.validator.ValidatePrediction(actualResult.Qihao, actualResult)
	if err != nil {
		return fmt.Errorf("validation failed: %v", err)
	}

	// 更新缓存
	if err := a.cacheManager.OnPredictionVerified(actualResult.Qihao, validation.IsCorrect); err != nil {
		logger.Warnf("Failed to update cache for prediction verification: %v", err)
	}

	logger.Infof("Prediction verified for %s: %s", actualResult.Qihao,
		map[bool]string{true: "CORRECT", false: "INCORRECT"}[validation.IsCorrect])

	return nil
}

// generateNewPrediction 生成新预测
func (a *App) generateNewPrediction() error {
	// 获取历史数据用于预测
	historyData, err := a.cacheManager.GetLast3LotteryData()
	if err != nil {
		return fmt.Errorf("failed to get history data for prediction: %v", err)
	}

	if len(historyData) < 3 {
		return fmt.Errorf("insufficient history data for prediction: need 3, got %d", len(historyData))
	}

	// 生成预测
	predictionResult, err := a.predictorMgr.Predict(historyData)
	if err != nil {
		return fmt.Errorf("prediction generation failed: %v", err)
	}

	// 计算预测和值和单双
	predictedNums, _ := database.ParseOpenNum(predictionResult.PredictedNum)
	predictedSum := database.CalculateSum(predictedNums)
	predictedOddEven := database.CalculateOddEven(predictedSum)

	// 保存预测到数据库
	prediction := &database.Prediction{
		TargetQihao:      predictionResult.TargetQihao,
		PredictedNum:     predictionResult.PredictedNum,
		PredictedSum:     predictedSum,
		PredictedOddEven: predictedOddEven,
		ConfidenceScore:  nil, // 不使用置信度
		AlgorithmVersion: predictionResult.AlgorithmVersion,
		PredictedAt:      predictionResult.Timestamp,
	}

	if err := a.mysql.SavePrediction(prediction); err != nil {
		return fmt.Errorf("failed to save prediction: %v", err)
	}

	// 更新缓存
	if err := a.cacheManager.OnPredictionGenerated(prediction); err != nil {
		logger.Warnf("Failed to update cache for new prediction: %v", err)
	}

	// 广播新预测（如果有订阅用户）
	latestResult, _ := a.cacheManager.GetLatestLotteryData()
	if err := a.telegramBot.BroadcastNewPrediction(prediction, latestResult); err != nil {
		logger.Warnf("Failed to broadcast new prediction: %v", err)
	}

	fmt.Printf("🔮 生成预测: %s -> %s (固定算法)\n",
		prediction.TargetQihao, prediction.PredictedNum)

	return nil
}

// dataCleanupLoop 数据清理循环
func (a *App) dataCleanupLoop() {
	defer a.wg.Done()

	// 每小时执行一次清理
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := a.mysql.CleanOldData(); err != nil {
				fmt.Printf("❌ 数据清理失败: %v\n", err)
			} else {
				fmt.Println("🧹 定期数据清理完成")
			}
		case <-a.stopChannel:
			return
		}
	}
}

// HealthCheck 健康检查
func (a *App) HealthCheck() map[string]interface{} {
	health := map[string]interface{}{
		"timestamp": time.Now(),
		"status":    "ok",
		"services":  map[string]interface{}{},
	}

	services := health["services"].(map[string]interface{})

	// 检查API健康状态
	if err := a.apiClient.HealthCheck(); err != nil {
		services["api"] = map[string]interface{}{
			"status": "error",
			"error":  err.Error(),
		}
		health["status"] = "degraded"
	} else {
		services["api"] = map[string]interface{}{
			"status": "ok",
		}
	}

	// 检查缓存状态
	cacheStats := a.cacheManager.GetStats()
	services["cache"] = map[string]interface{}{
		"status": "ok",
		"stats":  cacheStats,
	}

	// 检查Telegram Bot状态
	botInfo := a.telegramBot.GetBotInfo()
	services["telegram"] = map[string]interface{}{
		"status": "ok",
		"info":   botInfo,
	}

	return health
}

func main() {
	// 配置文件路径
	configPath := "configs/config.yaml"
	if len(os.Args) > 1 {
		configPath = os.Args[1]
	}

	// 创建应用程序实例
	app, err := NewApp(configPath)
	if err != nil {
		fmt.Printf("❌ 应用初始化失败: %v\n", err)
		os.Exit(1)
	}

	// 启动应用程序
	if err := app.Start(); err != nil {
		fmt.Printf("❌ 应用启动失败: %v\n", err)
		os.Exit(1)
	}

	// 设置信号处理
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// 等待停止信号
	<-sigChan

	// 优雅关闭
	if err := app.Stop(); err != nil {
		fmt.Printf("❌ 关闭时出错: %v\n", err)
		os.Exit(1)
	}
}
