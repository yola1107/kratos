package config

import (
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
	QueueSize   int           // 队列大小
	MaxInterval time.Duration // 发送间隔
	MaxBatchCnt int           // 最大批量数
	MaxRetries  int           // 最大重试
	Telegram    Telegram
}

type Telegram struct {
	Enabled bool
	Token   string
	ChatID  string
}

func DefaultConfig() *Config {
	return &Config{
		Mode:          Development,
		Level:         "debug",
		Directory:     "./logs",
		Filename:      "app.log",
		ErrorFilename: "error.log",
		MaxSize:       50,
		MaxAge:        7,
		MaxBackups:    3,
		FlushInterval: 1 * time.Second,
		Compress:      false,
		LocalTime:     true,
		QueueSize:     512,
		PoolSize:      128,
		//SensitiveKeys: []string{"password", "token", "secret"},
		Alert: &Alert{
			Enabled:     false,
			Threshold:   zapcore.ErrorLevel,
			QueueSize:   100,
			MaxInterval: 3 * time.Second,
			MaxBatchCnt: 10,
			MaxRetries:  1,
			Telegram: Telegram{
				Enabled: false,
				Token:   "",
				ChatID:  "",
			},
		},
	}
}
