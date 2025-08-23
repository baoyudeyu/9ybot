package telegram

import (
	"fmt"

	"pc28-bot/internal/cache"
	"pc28-bot/internal/config"
	"pc28-bot/internal/database"
	"pc28-bot/internal/logger"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// Bot Telegram机器人
type Bot struct {
	api           *tgbotapi.BotAPI
	cacheManager  *cache.CacheManager
	updateChannel tgbotapi.UpdatesChannel
	stopChannel   chan bool
}

// NewBot 创建新的Telegram机器人
func NewBot(cfg *config.Telegram, cacheManager *cache.CacheManager) (*Bot, error) {
	bot, err := tgbotapi.NewBotAPI(cfg.Token)
	if err != nil {
		return nil, fmt.Errorf("failed to create telegram bot: %v", err)
	}

	bot.Debug = false
	logger.Infof("Telegram bot authorized on account: %s", bot.Self.UserName)

	// 配置更新
	u := tgbotapi.NewUpdate(0)
	u.Timeout = int(cfg.Timeout.Seconds())

	updates := bot.GetUpdatesChan(u)

	return &Bot{
		api:           bot,
		cacheManager:  cacheManager,
		updateChannel: updates,
		stopChannel:   make(chan bool),
	}, nil
}

// Start 启动机器人
func (b *Bot) Start() {
	logger.Info("Starting Telegram bot...")

	go b.handleUpdates()
	logger.Info("Telegram bot started successfully")
}

// Stop 停止机器人
func (b *Bot) Stop() {
	logger.Info("Stopping Telegram bot...")
	b.stopChannel <- true
	b.api.StopReceivingUpdates()
	logger.Info("Telegram bot stopped")
}

// handleUpdates 处理更新
func (b *Bot) handleUpdates() {
	for {
		select {
		case update := <-b.updateChannel:
			if update.Message != nil {
				// 只处理私聊消息，忽略群组消息
				if update.Message.Chat.IsPrivate() {
					go b.handleMessage(update.Message)
				}
			} else if update.CallbackQuery != nil {
				// 只处理私聊中的回调查询
				if update.CallbackQuery.Message.Chat.IsPrivate() {
					go b.handleCallbackQuery(update.CallbackQuery)
				}
			}
		case <-b.stopChannel:
			return
		}
	}
}

// handleMessage 处理消息
func (b *Bot) handleMessage(message *tgbotapi.Message) {
	// 再次确认是私聊消息
	if !message.Chat.IsPrivate() {
		return
	}

	if message.IsCommand() {
		b.handleCommand(message)
	} else {
		b.handleTextMessage(message)
	}
}

// handleCommand 处理命令
func (b *Bot) handleCommand(message *tgbotapi.Message) {
	// 确保只在私聊中处理命令
	if !message.Chat.IsPrivate() {
		return
	}

	command := message.Command()
	chatID := message.Chat.ID

	logger.Debugf("Received private command: %s from user: %d", command, chatID)

	switch command {
	case "start":
		b.handleStartCommand(chatID)
	case "help":
		b.handleHelpCommand(chatID)
	case "latest":
		b.handleLatestCommand(chatID)
	case "history":
		b.handleHistoryCommand(chatID)
	case "stats":
		b.handleStatsCommand(chatID)
	// 移除了 prediction 命令
	default:
		b.sendMessage(chatID, "Unknown command. Type /help to view available commands.")
	}
}

// handleStartCommand 处理开始命令
func (b *Bot) handleStartCommand(chatID int64) {
	welcomeText := `🎮 Welcome to PC28 Prediction Bot!

🤖 I am your intelligent prediction assistant, providing you with:
• 📊 Latest lottery results
• 🔮 Smart prediction results  
• 📈 Historical prediction records
• 📊 Accuracy statistics

📝 Available commands:
/latest - View latest predictions
/history - View lottery records
/stats - View statistics
/help - Help information

⚠️ Note: This bot only provides services in private chats
🔔 The bot will automatically push the latest prediction results!`

	b.sendMessage(chatID, welcomeText)
}

// handleHelpCommand 处理帮助命令
func (b *Bot) handleHelpCommand(chatID int64) {
	helpText := `📖 Command Help:

/start - Start using the bot
/latest - Get latest prediction results
/history - View recent 10 lottery records
/stats - View prediction accuracy statistics
/help - Show this help information

💡 Usage Tips:
• Bot automatically analyzes latest data each round
• Based on recent 3 historical data for prediction
• Prediction results are for reference only, please be rational

📞 If you have any questions, please contact the administrator.`

	b.sendMessage(chatID, helpText)
}

// handleLatestCommand 处理最新命令
func (b *Bot) handleLatestCommand(chatID int64) {
	// 获取预测历史记录（10期历史 + 1期最新预测 = 11期）
	predictionHistory, err := b.cacheManager.GetPredictionHistory(11)
	if err != nil {
		b.sendMessage(chatID, "❌ Failed to get prediction records, please try again later.")
		logger.Errorf("Failed to get prediction history: %v", err)
		return
	}

	// 格式化消息（使用新的单双预测模板）
	message := b.formatPredictionHistoryMessage(predictionHistory)
	b.sendMessage(chatID, message)
}

// handleHistoryCommand 处理历史命令
func (b *Bot) handleHistoryCommand(chatID int64) {
	// 获取历史开奖记录
	lotteryHistory, err := b.cacheManager.GetLotteryHistory(10)
	if err != nil {
		b.sendMessage(chatID, "❌ Failed to get history records, please try again later.")
		logger.Errorf("Failed to get lottery history: %v", err)
		return
	}

	// 格式化消息
	message := b.formatLotteryHistoryMessage(lotteryHistory)
	b.sendMessage(chatID, message)
}

// handleStatsCommand 处理统计命令
func (b *Bot) handleStatsCommand(chatID int64) {
	// 获取统计信息
	stats, err := b.cacheManager.GetPredictionStats()
	if err != nil {
		b.sendMessage(chatID, "❌ Failed to get statistics, please try again later.")
		logger.Errorf("Failed to get prediction stats: %v", err)
		return
	}

	// 格式化消息
	message := b.formatStatsMessage(stats)
	b.sendMessage(chatID, message)
}

// 移除了 handlePredictionCommand 函数

// handleTextMessage 处理文本消息
func (b *Bot) handleTextMessage(message *tgbotapi.Message) {
	// 确保只在私聊中处理文本消息
	if !message.Chat.IsPrivate() {
		return
	}

	chatID := message.Chat.ID
	text := message.Text

	// 简单的智能回复
	switch text {
	case "最新", "最新数据":
		b.handleLatestCommand(chatID)
	case "历史", "历史记录":
		b.handleHistoryCommand(chatID)
	case "统计", "准确率":
		b.handleStatsCommand(chatID)
	// 移除了预测相关的文本命令
	default:
		b.sendMessage(chatID, "Please use commands or keywords, type /help for help.")
	}
}

// handleCallbackQuery 处理回调查询
func (b *Bot) handleCallbackQuery(callback *tgbotapi.CallbackQuery) {
	// 确保只在私聊中处理回调查询
	if !callback.Message.Chat.IsPrivate() {
		return
	}

	chatID := callback.Message.Chat.ID
	data := callback.Data

	logger.Debugf("Received private callback: %s from user: %d", data, chatID)

	switch data {
	case "refresh_latest":
		b.handleLatestCommand(chatID)
	case "view_history":
		b.handleHistoryCommand(chatID)
	case "view_stats":
		b.handleStatsCommand(chatID)
	}

	// 应答回调查询
	callbackResponse := tgbotapi.NewCallback(callback.ID, "")
	b.api.Request(callbackResponse)
}

// sendMessage 发送消息（仅发送给私聊）
func (b *Bot) sendMessage(chatID int64, text string) {
	// 确保只向私聊用户发送消息（正数ID）
	if chatID < 0 {
		logger.Debugf("Skipping message to group chat %d", chatID)
		return
	}

	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = tgbotapi.ModeMarkdown

	_, err := b.api.Send(msg)
	if err != nil {
		logger.Errorf("Failed to send message to user %d: %v", chatID, err)
	}
}

// BroadcastNewPrediction 广播新预测结果（仅发送给私聊用户）
func (b *Bot) BroadcastNewPrediction(prediction *database.Prediction, actualResult *database.LotteryResult) error {
	message := b.formatNewPredictionBroadcast(prediction, actualResult)

	// 获取私聊订阅用户列表
	subscribedUsers := b.getSubscribedUsers()

	for _, userID := range subscribedUsers {
		// 确保只向私聊用户发送
		if userID > 0 { // 正数ID表示用户，负数ID表示群组
			b.sendMessage(userID, message)
		}
	}

	logger.Infof("Broadcasted new prediction to %d private users", len(subscribedUsers))
	return nil
}

// getSubscribedUsers 获取订阅的私聊用户列表
func (b *Bot) getSubscribedUsers() []int64 {
	// 这里应该从数据库获取已订阅的私聊用户ID列表
	// 目前返回空列表，实际使用时需要实现用户订阅功能
	// 注意：只返回正数的用户ID，不包含群组ID（负数）
	return []int64{}
}

// GetBotInfo 获取机器人信息
func (b *Bot) GetBotInfo() map[string]interface{} {
	return map[string]interface{}{
		"username":        b.api.Self.UserName,
		"id":              b.api.Self.ID,
		"first_name":      b.api.Self.FirstName,
		"is_bot":          b.api.Self.IsBot,
		"can_join_groups": b.api.Self.CanJoinGroups,
	}
}
