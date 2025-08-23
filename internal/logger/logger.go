package logger

import (
	"os"

	"github.com/sirupsen/logrus"
)

var Log *logrus.Logger

// InitLogger 初始化日志器
func InitLogger(level string) {
	Log = logrus.New()
	
	// 设置输出格式
	Log.SetFormatter(&logrus.TextFormatter{
		FullTimestamp:   true,
		TimestampFormat: "2006-01-02 15:04:05",
	})
	
	// 设置输出到标准输出
	Log.SetOutput(os.Stdout)
	
	// 设置日志级别
	switch level {
	case "debug":
		Log.SetLevel(logrus.DebugLevel)
	case "info":
		Log.SetLevel(logrus.InfoLevel)
	case "warn":
		Log.SetLevel(logrus.WarnLevel)
	case "error":
		Log.SetLevel(logrus.ErrorLevel)
	default:
		Log.SetLevel(logrus.InfoLevel)
	}
}

// Debug 调试日志
func Debug(args ...interface{}) {
	Log.Debug(args...)
}

// Debugf 格式化调试日志
func Debugf(format string, args ...interface{}) {
	Log.Debugf(format, args...)
}

// Info 信息日志
func Info(args ...interface{}) {
	Log.Info(args...)
}

// Infof 格式化信息日志
func Infof(format string, args ...interface{}) {
	Log.Infof(format, args...)
}

// Warn 警告日志
func Warn(args ...interface{}) {
	Log.Warn(args...)
}

// Warnf 格式化警告日志
func Warnf(format string, args ...interface{}) {
	Log.Warnf(format, args...)
}

// Error 错误日志
func Error(args ...interface{}) {
	Log.Error(args...)
}

// Errorf 格式化错误日志
func Errorf(format string, args ...interface{}) {
	Log.Errorf(format, args...)
}

// Fatal 致命错误日志
func Fatal(args ...interface{}) {
	Log.Fatal(args...)
}

// Fatalf 格式化致命错误日志
func Fatalf(format string, args ...interface{}) {
	Log.Fatalf(format, args...)
}

