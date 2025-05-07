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
	Enabled     bool
	Threshold   zapcore.Level // 触发日志级别
	QueueSize   int           // 队列大小
	MaxInterval time.Duration // 发送间隔
	MaxBatchCnt int           // 最大批量数
	MaxRetries  int           // 最大重试
	Telegram    Telegram

	//Batch     Batch
	//RateLimit RateLimit
}

//type Batch struct {
//	Enabled     bool
//	MaxSize     int
//	MaxInterval time.Duration
//	QueueSize   int
//}
//
//type RateLimit struct {
//	Enabled  bool
//	Interval time.Duration // 时间间隔
//	Burst    int           // 突发数量
//}

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
			Enabled: false,
			//Batch: Batch{
			//	Enabled:     false,
			//	MaxSize:     0,
			//	MaxInterval: 0,
			//},
			//RateLimit: RateLimit{
			//	Enabled:  false,
			//	Interval: 0,
			//	Burst:    0,
			//},
			Threshold: zapcore.ErrorLevel,
			Telegram: Telegram{
				Enabled: false,
				Token:   "",
				ChatID:  "",
			},
		},
	}
}

//type (
//	Config2 struct {
//		QueueSize   int            `yaml:"queue_size"`    // 队列大小
//		RateLimit   time.Duration  `yaml:"rate_limit"`    // 发送间隔
//		MaxBatchCnt int            `yaml:"max_batch_cnt"` // 最大批量数
//		MaxRetries  int            `yaml:"max_retries"`   // 最大重试
//		Telegram    TelegramConfig `yaml:"telegram"`
//	}
//	TelegramConfig struct {
//		Enabled   bool          `yaml:"enabled"`   // 是否启用
//		Threshold zapcore.Level `yaml:"threshold"` // 日志级别
//		Token     string        `yaml:"token"`     // Bot Token
//		ChatID    string        `yaml:"chat_id"`   // 聊天ID
//		Prefix    string        `yaml:"prefix"`    // 消息前缀
//	}
//)
