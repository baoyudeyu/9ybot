# PC28预测机器人

基于Go语言开发的PC28彩票预测机器人，集成MySQL数据库存储、Redis缓存加速、智能预测算法和Telegram Bot交互。

## 🎯 功能特性

- **智能预测算法**: 基于历史数据分析，提供智能预测
- **实时数据监控**: 每秒监控开奖API，实时获取最新数据
- **高效缓存系统**: 优化的内存缓存，确保高性能响应
- **私聊专用Bot**: 仅在Telegram私聊中提供服务，保护隐私
- **预测验证统计**: 自动验证预测准确率并提供统计分析
- **简化架构**: 精简的代码结构，易于维护和部署

## 📋 系统要求

- Go 1.21+
- MySQL 5.7+
- Redis 6.0+
- 稳定的网络连接

## 🚀 快速开始

### 1. 克隆项目
```bash
git clone <repository-url>
cd pc28-bot
```

### 2. 安装依赖
```bash
go mod tidy
```

### 3. 配置数据库
```bash
# 连接到MySQL数据库
mysql -h gz-cynosdbmysql-grp-jj6na063.sql.tencentcdb.com -P 21151 -u y9 -p

# 执行初始化脚本
source scripts/init.sql
```

### 4. 配置Redis (可选)
如果使用本地Redis，请确保Redis服务正在运行：
```bash
redis-server
```

### 5. 配置文件
编辑 `configs/config.yaml` 中的配置信息（如Redis地址等）

### 6. 运行程序

**推荐方式**（自动启动Redis）：
```bash
.\run.bat
```

或直接运行编译好的程序：
```bash
.\pc28-bot.exe
```

## 📁 项目结构

```
pc28-bot/
├── cmd/
│   └── main.go                 # 应用程序入口
├── internal/
│   ├── api/                    # API客户端
│   │   └── client.go
│   ├── cache/                  # 缓存模块
│   │   ├── manager.go
│   │   ├── memory.go
│   │   └── redis.go
│   ├── database/               # 数据库模块
│   │   ├── models.go
│   │   └── mysql.go
│   ├── predictor/              # 预测算法
│   │   ├── interface.go
│   │   ├── algorithm.go
│   │   └── validator.go
│   ├── telegram/               # Telegram Bot
│   │   ├── bot.go
│   │   └── templates.go
│   ├── config/                 # 配置管理
│   │   └── config.go
│   └── logger/                 # 日志模块
│       └── logger.go
├── configs/
│   └── config.yaml             # 配置文件
├── scripts/
│   └── init.sql                # 数据库初始化脚本
├── go.mod
├── go.sum
└── README.md
```

## 🔧 配置说明

### 数据库配置
```yaml
database:
  host: "gz-cynosdbmysql-grp-jj6na063.sql.tencentcdb.com"
  port: 21151
  username: "y9"
  database: "y9"
  password: "04By0302"
```

### Telegram Bot配置
```yaml
telegram:
  token: "7934063071:AAFkVhynDSPf_VeIVlneWyXaRY1qVPBULxs"
  timeout: "30s"
```

### API配置
```yaml
api:
  url: "https://pc28.help/kj.json"
  timeout: "10s"
  retry_count: 3
```

## 🤖 Telegram Bot 命令

⚠️ **重要提醒**: 机器人仅在私聊中工作，不会响应群组消息

- `/start` - 开始使用机器人
- `/latest` - 查看最新预测和开奖信息
- `/history` - 查看最近10期开奖记录（≥14为大）
- `/stats` - 查看预测准确率统计
- `/prediction` - 获取下期预测详情
- `/help` - 显示帮助信息

支持关键词快捷操作：发送"最新"、"历史"、"统计"、"预测"等关键词

## 📊 预测算法

当前实现了基于频率分析的默认预测算法，分析最近3期的历史数据：

1. **频率统计**: 统计每个位置数字的出现频率
2. **趋势分析**: 分析数字变化趋势
3. **置信度计算**: 基于数据一致性计算预测置信度
4. **结果验证**: 自动验证预测准确性

### 自定义算法
您可以通过实现 `Predictor` 接口来添加自定义预测算法：

```go
type Predictor interface {
    Predict(history []database.LotteryResult) (*database.PredictionResult, error)
    GetName() string
    GetVersion() string
    ValidateInput(history []database.LotteryResult) error
    GetRequiredHistorySize() int
}
```

## 🔄 数据流程

1. **数据监控**: 每秒调用PC28 API获取最新开奖数据
2. **新数据检测**: 检查是否有新期号的开奖结果
3. **预测验证**: 验证之前预测的准确性
4. **缓存更新**: 更新内存缓存中的数据
5. **生成预测**: 基于最新数据生成下期预测
6. **私聊推送**: 通过Telegram Bot向私聊用户推送预测结果

## 🔒 隐私保护

- **私聊专用**: 机器人只在私聊中工作，完全忽略群组消息
- **无群组交互**: 不会读取或响应任何群组中的消息
- **用户专属**: 每个用户独立获取预测服务

## 📈 监控和维护

### 健康检查
系统提供健康检查功能，监控各组件状态：
- API连接状态
- 数据库连接状态
- Redis连接状态
- Telegram Bot状态

### 数据清理
系统自动清理超过24小时的历史数据，保持数据库性能。

### 日志管理
支持多级别日志记录：
- DEBUG: 详细调试信息
- INFO: 一般信息记录
- WARN: 警告信息
- ERROR: 错误信息

## ⚠️ 免责声明

本软件仅供学习和研究使用，预测结果不构成任何投资建议。彩票具有随机性，任何预测算法都无法保证100%准确。请理性对待预测结果，不要过度依赖。

## 🤝 贡献

欢迎提交Issue和Pull Request来改进这个项目。

## 📄 许可证

本项目采用MIT许可证。

## 📞 支持

如有问题或建议，请通过以下方式联系：
- 创建GitHub Issue
- 发送邮件到项目维护者

---

**注意**: 请确保遵守当地法律法规，本项目不承担任何法律责任。
