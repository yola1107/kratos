package config

import (
	"os"
	"time"

	"go.uber.org/zap/zapcore"
)

type Mode string

const (
	Development Mode = "dev"
	Production  Mode = "prod"
)

type Config struct {
	Mode          Mode
	Level         string
	Directory     string
	Filename      string
	ErrorFilename string
	MaxSize       int
	MaxAge        int
	MaxBackups    int
	FlushInterval time.Duration
	Compress      bool
	QueueSize     int
	PoolSize      int
	LocalTime     bool

	Alert *Alert
}

type Alert struct {
	Enabled     bool
	Threshold   zapcore.Level // 触发日志级别
	MaxInterval time.Duration // 发送间隔
	QueueSize   int           // 队列大小
	MaxBatchCnt int           // 最大批量数
	MaxRetries  int           // 最大重试
	Prefix      string        // 消息前缀
	Telegram    *Telegram
}

type Telegram struct {
	Token  string
	ChatID string
}

func DefaultConfig() *Config {
	m := os.Getenv("APP_LOGGER_MODE")

	switch Mode(m) {
	case Production:
		return &Config{
			Mode:          Production,
			Level:         "info", // 生产环境默认info级别
			Directory:     "/var/log/app",
			Filename:      "app.log",
			ErrorFilename: "error.log",
			MaxSize:       200, // 单个日志文件最大200MB
			MaxAge:        7,   // 保留7天
			MaxBackups:    10,  // 保留10个备份
			FlushInterval: 3 * time.Second,
			Compress:      true, // 启用压缩
			LocalTime:     true,
			QueueSize:     2048, // 增大队列缓冲
			PoolSize:      512,  // 更大的对象池
			//SensitiveKeys: []string{"password", "token", "secret"},
			Alert: &Alert{
				Enabled:     true,
				Threshold:   zapcore.ErrorLevel,
				MaxInterval: 5 * time.Second,
				QueueSize:   2048,
				MaxBatchCnt: 20,
				MaxRetries:  1,
				Prefix:      "",
				Telegram: &Telegram{
					Token:  os.Getenv("TG_TOKEN"),
					ChatID: os.Getenv("TG_CHAT_ID"),
				},
			},
		}

	default:
		return &Config{
			Mode:          Development,
			Level:         "debug", // 开发环境更详细日志
			Directory:     "./logs",
			Filename:      "app.log",
			ErrorFilename: "error.log",
			MaxSize:       50, // 较小文件大小
			MaxAge:        7,  // 保留7天
			MaxBackups:    3,  // 保留3个备份
			FlushInterval: 1 * time.Second,
			Compress:      false, // 开发环境不压缩
			LocalTime:     true,
			QueueSize:     512, // 较小队列
			PoolSize:      128,
			//SensitiveKeys: []string{"password", "token", "secret"},
			Alert: &Alert{
				Enabled:     false,
				Threshold:   zapcore.ErrorLevel,
				MaxInterval: 1 * time.Second,
				QueueSize:   100,
				MaxBatchCnt: 10,
				MaxRetries:  1,
				Prefix:      "",
				Telegram: &Telegram{
					Token:  os.Getenv("TG_TOKEN"),
					ChatID: os.Getenv("TG_CHAT_ID"),
					//Token:  "TG_TOKEN",
					//ChatID: "TG_CHAT_ID",
				},
			},
		}
	}
}
