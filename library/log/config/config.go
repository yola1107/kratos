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

	Alert *Alert `yaml:"alert" json:"alert"`
}

type Alert struct {
	Enabled   bool
	Batch     Batch
	RateLimit RateLimit
	Telegram  Telegram
}

type Batch struct {
	Enabled     bool
	MaxSize     int
	MaxInterval time.Duration
}

type RateLimit struct {
	Enabled  bool
	Interval time.Duration // 时间间隔
	Burst    int           // 突发数量
}

type Telegram struct {
	Enabled   bool
	Token     string
	ChatID    string
	Threshold zapcore.Level // 触发日志级别
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
			Enabled: false,
			Batch: Batch{
				Enabled:     false,
				MaxSize:     0,
				MaxInterval: 0,
			},
			RateLimit: RateLimit{
				Enabled:  false,
				Interval: 0,
				Burst:    0,
			},
			Telegram: Telegram{
				Enabled:   false,
				Token:     "",
				ChatID:    "",
				Threshold: zapcore.ErrorLevel,
			},
		},
	}
}
