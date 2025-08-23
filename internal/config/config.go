package config

import (
	"fmt"
	"io/ioutil"
	"time"

	"gopkg.in/yaml.v2"
)

// Config 应用程序配置结构
type Config struct {
	Database Database `yaml:"database"`
	Telegram Telegram `yaml:"telegram"`
	API      API      `yaml:"api"`
	App      App      `yaml:"app"`
}

// Database 数据库配置
type Database struct {
	Host            string        `yaml:"host"`
	Port            int           `yaml:"port"`
	Username        string        `yaml:"username"`
	Database        string        `yaml:"database"`
	Password        string        `yaml:"password"`
	MaxOpenConns    int           `yaml:"max_open_conns"`
	MaxIdleConns    int           `yaml:"max_idle_conns"`
	ConnMaxLifetime time.Duration `yaml:"conn_max_lifetime"`
}

// Telegram Bot配置
type Telegram struct {
	Token   string        `yaml:"token"`
	Timeout time.Duration `yaml:"timeout"`
}

// API 外部API配置
type API struct {
	URL        string        `yaml:"url"`
	Timeout    time.Duration `yaml:"timeout"`
	RetryCount int           `yaml:"retry_count"`
	RetryDelay time.Duration `yaml:"retry_delay"`
}

// App 应用程序配置
type App struct {
	PollingInterval    time.Duration `yaml:"polling_interval"`
	DataRetentionHours int           `yaml:"data_retention_hours"`
	LogLevel           string        `yaml:"log_level"`
	CacheTTL           time.Duration `yaml:"cache_ttl"`
}

// LoadConfig 加载配置文件
func LoadConfig(configPath string) (*Config, error) {
	data, err := ioutil.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %v", err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %v", err)
	}

	return &config, nil
}

// GetDSN 获取数据库连接字符串
func (d *Database) GetDSN() string {
	return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		d.Username, d.Password, d.Host, d.Port, d.Database)
}
