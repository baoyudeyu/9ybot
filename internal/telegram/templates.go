package telegram

import (
	"fmt"
	"strings"
	"time"

	"pc28-bot/internal/database"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// formatLatestDataMessage 格式化最新数据消息
func (b *Bot) formatLatestDataMessage(latest *database.LotteryResult, prediction *database.Prediction) string {
	var builder strings.Builder

	// 标题
	builder.WriteString("📊 *Latest Lottery Information*\n\n")

	// 最新开奖数据
	builder.WriteString("🎯 *Latest Result*\n")
	builder.WriteString(fmt.Sprintf("Round: `%s`\n", latest.Qihao))
	builder.WriteString(fmt.Sprintf("Numbers: `%s`\n", latest.OpenNum))
	builder.WriteString(fmt.Sprintf("Sum: `%d`\n", latest.SumValue))
	builder.WriteString(fmt.Sprintf("Time: `%s`\n\n", latest.OpenTime.Format("01-02 15:04:05")))

	// 最新预测信息
	if prediction != nil {
		builder.WriteString("🔮 *Latest Prediction*\n")
		builder.WriteString(fmt.Sprintf("Target Round: `%s`\n", prediction.TargetQihao))
		builder.WriteString(fmt.Sprintf("Predicted Numbers: `%s`\n", prediction.PredictedNum))

		// 移除置信度显示

		builder.WriteString(fmt.Sprintf("Prediction Time: `%s`\n", prediction.PredictedAt.Format("01-02 15:04:05")))

		// 如果已验证，显示结果
		if prediction.IsCorrect != nil {
			if *prediction.IsCorrect {
				builder.WriteString("✅ *Prediction Correct*\n")
			} else {
				builder.WriteString("❌ *Prediction Wrong*\n")
			}
		}
	} else {
		builder.WriteString("🔮 *No Prediction Data*\n")
	}

	builder.WriteString("\n💡 *Tips*: Predictions are for reference only, please be rational")

	return builder.String()
}

// formatHistoryMessage 格式化历史记录消息
func (b *Bot) formatHistoryMessage(predictions []database.Prediction) string {
	var builder strings.Builder

	builder.WriteString("📈 *最近10期预测记录*\n\n")

	if len(predictions) == 0 {
		builder.WriteString("暂无预测记录")
		return builder.String()
	}

	// 统计准确率
	correctCount := 0
	verifiedCount := 0

	for i, pred := range predictions {
		if i >= 10 { // 只显示最近10期
			break
		}

		// 序号和基本信息
		builder.WriteString(fmt.Sprintf("*%d.* 期号 `%s`\n", i+1, pred.TargetQihao))
		builder.WriteString(fmt.Sprintf("   预测: `%s`", pred.PredictedNum))

		// 实际结果和准确性
		if pred.ActualNum != nil && pred.IsCorrect != nil {
			builder.WriteString(fmt.Sprintf(" | 实际: `%s`", *pred.ActualNum))
			if *pred.IsCorrect {
				builder.WriteString(" ✅")
				correctCount++
			} else {
				builder.WriteString(" ❌")
			}
			verifiedCount++
		} else {
			builder.WriteString(" | 待验证 ⏳")
		}

		builder.WriteString("\n")

		// 移除置信度显示

		builder.WriteString("\n")
	}

	// 总体统计
	if verifiedCount > 0 {
		accuracy := float64(correctCount) / float64(verifiedCount) * 100
		builder.WriteString(fmt.Sprintf("📊 *近期准确率*: %.1f%% (%d/%d)\n", accuracy, correctCount, verifiedCount))
	}

	return builder.String()
}

// formatPredictionHistoryMessage 格式化预测历史消息（新模板）
func (b *Bot) formatPredictionHistoryMessage(predictions []database.Prediction) string {
	var builder strings.Builder

	builder.WriteString("📊 Recent 10 Prediction Records\n\n")

	if len(predictions) == 0 {
		builder.WriteString("No prediction records")
		return builder.String()
	}

	// 限制显示数量：10期历史 + 1期最新预测 = 11期
	displayCount := len(predictions)
	if displayCount > 11 {
		displayCount = 11
	}

	// 从最老的开始显示到最新的（反转顺序）
	for i := displayCount - 1; i >= 0; i-- {
		pred := predictions[i]

		// 显示格式按照用户模板：Round 3326098 Even丨Result：4+3+0=7 Wrong❌
		if pred.ActualOddEven != nil && pred.IsCorrect != nil {
			// 已开奖的期号
			result := "Correct✅"
			if !*pred.IsCorrect {
				result = "Wrong❌"
			}
			// 翻译预测的单双
			predictedOddEvenEN := b.translateOddEven(pred.PredictedOddEven)
			builder.WriteString(fmt.Sprintf("Round %s %s丨Result：%s=%d %s\n",
				pred.TargetQihao, predictedOddEvenEN, *pred.ActualNum, *pred.ActualSum, result))
		} else {
			// 待开奖的期号（最新预测）
			predictedOddEvenEN := b.translateOddEven(pred.PredictedOddEven)
			builder.WriteString(fmt.Sprintf("Round %s %s丨Pending\n",
				pred.TargetQihao, predictedOddEvenEN))
		}
	}

	// 计算准确率
	correctCount := 0
	verifiedCount := 0
	for i := 0; i < displayCount; i++ {
		pred := predictions[i]
		if pred.IsCorrect != nil {
			verifiedCount++
			if *pred.IsCorrect {
				correctCount++
			}
		}
	}

	builder.WriteString("\n")
	if verifiedCount > 0 {
		accuracy := float64(correctCount) / float64(verifiedCount) * 100
		builder.WriteString(fmt.Sprintf("📈 Recent Accuracy: %.1f%% (%d/%d)", accuracy, correctCount, verifiedCount))
	} else {
		builder.WriteString("📈 Recent Accuracy: 0.0% (0/0)")
	}

	return builder.String()
}

// formatLotteryHistoryMessage 格式化历史开奖消息
func (b *Bot) formatLotteryHistoryMessage(lotteryHistory []database.LotteryResult) string {
	var builder strings.Builder

	builder.WriteString("📊 *Recent 10 Lottery Records*\n\n")

	if len(lotteryHistory) == 0 {
		builder.WriteString("No lottery records")
		return builder.String()
	}

	// 限制显示数量并反转顺序（最新的在最下面）
	displayCount := len(lotteryHistory)
	if displayCount > 10 {
		displayCount = 10
	}

	// 从最老的开始显示到最新的
	for i := displayCount - 1; i >= 0; i-- {
		result := lotteryHistory[i]

		// 解析开奖号码并计算大小
		sizePattern := "Small"
		if result.SumValue >= 14 {
			sizePattern = "Big"
		}

		// 解析单双
		oddEvenPattern := "Even"
		if result.SumValue%2 == 1 {
			oddEvenPattern = "Odd"
		}

		// 显示格式：Round 3326077
		//          Numbers: 3+1+0=4 (Small Even)
		//          Time: 08-23 10:15
		builder.WriteString(fmt.Sprintf("Round `%s`\n", result.Qihao))
		builder.WriteString(fmt.Sprintf("   Numbers: `%s=%d` (%s %s)\n", result.OpenNum, result.SumValue, sizePattern, oddEvenPattern))
		builder.WriteString(fmt.Sprintf("   Time: `%s`\n", result.OpenTimeString))
		builder.WriteString("\n")
	}

	// 统计大小分布
	bigCount := 0
	smallCount := 0
	for i := 0; i < displayCount; i++ {
		result := lotteryHistory[i]
		if result.SumValue >= 14 {
			bigCount++
		} else {
			smallCount++
		}
	}

	builder.WriteString(fmt.Sprintf("📈 *Recent Statistics*: Big %d rounds, Small %d rounds", bigCount, smallCount))

	return builder.String()
}

// formatStatsMessage 格式化统计信息消息
func (b *Bot) formatStatsMessage(stats *database.PredictionStats) string {
	var builder strings.Builder

	builder.WriteString("📊 *Prediction Statistics*\n\n")

	// 基本统计
	builder.WriteString("🎯 *Overall Performance*\n")
	builder.WriteString(fmt.Sprintf("Total Predictions: `%d`\n", stats.TotalPredictions))
	builder.WriteString(fmt.Sprintf("Correct Predictions: `%d`\n", stats.CorrectPredictions))
	builder.WriteString(fmt.Sprintf("Wrong Predictions: `%d`\n", stats.TotalPredictions-stats.CorrectPredictions))
	builder.WriteString(fmt.Sprintf("Overall Accuracy: `%.2f%%`\n\n", stats.AccuracyRate))

	// 时间信息
	if !stats.FirstPrediction.IsZero() {
		builder.WriteString("⏰ *Time Span*\n")
		builder.WriteString(fmt.Sprintf("First Prediction: `%s`\n", stats.FirstPrediction.Format("2006-01-02 15:04")))
		builder.WriteString(fmt.Sprintf("Latest Prediction: `%s`\n", stats.LastPrediction.Format("2006-01-02 15:04")))

		duration := stats.LastPrediction.Sub(stats.FirstPrediction)
		days := int(duration.Hours() / 24)
		builder.WriteString(fmt.Sprintf("Running Days: `%d days`\n\n", days))
	}

	// 评级系统
	rating := b.calculatePerformanceRating(stats.AccuracyRate)
	builder.WriteString(fmt.Sprintf("🏆 *Performance Rating*: %s\n\n", rating))

	// 提示信息
	builder.WriteString("💡 *Note*: Statistics are based on verified prediction results")

	return builder.String()
}

// 移除了 formatPredictionMessage 函数

// formatNewPredictionBroadcast 格式化新预测广播消息
func (b *Bot) formatNewPredictionBroadcast(prediction *database.Prediction, latestResult *database.LotteryResult) string {
	var builder strings.Builder

	builder.WriteString("🚨 *New Round Prediction Push*\n\n")

	// 最新开奖信息
	if latestResult != nil {
		builder.WriteString("📊 *Latest Result*\n")
		builder.WriteString(fmt.Sprintf("Round: `%s`\n", latestResult.Qihao))
		builder.WriteString(fmt.Sprintf("Numbers: `%s`\n", latestResult.OpenNum))
		builder.WriteString(fmt.Sprintf("Sum: `%d`\n\n", latestResult.SumValue))
	}

	// 新预测信息
	builder.WriteString("🔮 *Next Round Prediction*\n")
	builder.WriteString(fmt.Sprintf("Round: `%s`\n", prediction.TargetQihao))
	builder.WriteString(fmt.Sprintf("Numbers: `%s`\n", prediction.PredictedNum))

	// 移除置信度显示

	// 添加快捷按钮提示
	builder.WriteString("\n💡 Send /latest for details")

	return builder.String()
}

// formatVerificationMessage 格式化验证结果消息
func (b *Bot) formatVerificationMessage(qihao string, isCorrect bool, actualNum string, predictedNum string) string {
	var builder strings.Builder

	builder.WriteString("✅ *Prediction Verification Result*\n\n")

	builder.WriteString(fmt.Sprintf("Round: `%s`\n", qihao))
	builder.WriteString(fmt.Sprintf("Predicted Numbers: `%s`\n", predictedNum))
	builder.WriteString(fmt.Sprintf("Actual Numbers: `%s`\n", actualNum))

	if isCorrect {
		builder.WriteString("🎉 *Prediction Correct!*\n")
	} else {
		builder.WriteString("😅 *Prediction Wrong*\n")
	}

	return builder.String()
}

// calculatePerformanceRating 计算性能评级
func (b *Bot) calculatePerformanceRating(accuracy float64) string {
	switch {
	case accuracy >= 80:
		return "🏆 Excellent (≥80%)"
	case accuracy >= 70:
		return "🥇 Great (≥70%)"
	case accuracy >= 60:
		return "🥈 Good (≥60%)"
	case accuracy >= 50:
		return "🥉 Fair (≥50%)"
	default:
		return "📚 Needs Improvement (<50%)"
	}
}

// 移除了置信度等级函数

// analyzeSizePattern 分析大小形态
func (b *Bot) analyzeSizePattern(sum int) string {
	// PC28的和值范围通常是0-27
	if sum >= 14 {
		return "Big (≥14)"
	} else {
		return "Small (<14)"
	}
}

// analyzeOddEvenPattern 分析单双形态
func (b *Bot) analyzeOddEvenPattern(sum int) string {
	if sum%2 == 0 {
		return "Even"
	} else {
		return "Odd"
	}
}

// translateOddEven 翻译单双
func (b *Bot) translateOddEven(oddEven string) string {
	switch oddEven {
	case "单":
		return "Odd"
	case "双":
		return "Even"
	default:
		return oddEven // 如果已经是英文，直接返回
	}
}

// formatErrorMessage 格式化错误消息
func (b *Bot) formatErrorMessage(errorType string, details string) string {
	var builder strings.Builder

	builder.WriteString("❌ *System Error*\n\n")
	builder.WriteString(fmt.Sprintf("Error Type: `%s`\n", errorType))

	if details != "" {
		builder.WriteString(fmt.Sprintf("Details: `%s`\n", details))
	}

	builder.WriteString(fmt.Sprintf("Occurred Time: `%s`\n\n", time.Now().Format("2006-01-02 15:04:05")))
	builder.WriteString("Please try again later or contact the administrator")

	return builder.String()
}

// formatMaintenanceMessage 格式化维护消息
func (b *Bot) formatMaintenanceMessage(reason string, estimatedTime time.Duration) string {
	var builder strings.Builder

	builder.WriteString("🔧 *System Maintenance Notice*\n\n")
	builder.WriteString(fmt.Sprintf("Maintenance Reason: %s\n", reason))
	builder.WriteString(fmt.Sprintf("Estimated Duration: %s\n", estimatedTime.String()))
	builder.WriteString(fmt.Sprintf("Start Time: %s\n\n", time.Now().Format("2006-01-02 15:04:05")))
	builder.WriteString("Service may be temporarily unavailable during maintenance, please wait patiently")

	return builder.String()
}

// CreateInlineKeyboard 创建内联键盘
func (b *Bot) CreateInlineKeyboard() [][]tgbotapi.InlineKeyboardButton {
	return [][]tgbotapi.InlineKeyboardButton{
		{
			tgbotapi.NewInlineKeyboardButtonData("🔮 Latest Predictions", "refresh_latest"),
			tgbotapi.NewInlineKeyboardButtonData("📊 Lottery Records", "view_history"),
		},
		{
			tgbotapi.NewInlineKeyboardButtonData("📊 Statistics", "view_stats"),
		},
	}
}
