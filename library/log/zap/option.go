package zap

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"go.uber.org/zap/zapcore"
)

type Mode string

const (
	Development Mode = "dev"
	Production  Mode = "prod"
)

type Option func(*Config)

func WithDevelopment() Option {
	return func(c *Config) { c.Mode = Development }
}

func WithProduction() Option {
	return func(c *Config) { c.Mode = Production }
}

func WithLevel(level string) Option {
	return func(c *Config) { c.Level = level }
}

func WithDirectory(dir string) Option {
	return func(c *Config) { c.Directory = dir }
}

func WithFilename(filename string) Option {
	return func(c *Config) { c.Filename = filename }
}

func WithErrorFilename(filename string) Option {
	return func(c *Config) { c.ErrorFilename = filename }
}

func WithMaxSize(MaxSize int) Option {
	return func(c *Config) { c.MaxSize = MaxSize }
}

func WithMaxAge(MaxAge int) Option {
	return func(c *Config) { c.MaxAge = MaxAge }
}

func WithMaxBackups(maxBackups int) Option {
	return func(c *Config) { c.MaxBackups = maxBackups }
}

func WithQueueSize(queueSize int) Option {
	return func(c *Config) { c.QueueSize = queueSize }
}

func WithLocalTime(localTime bool) Option {
	return func(c *Config) { c.LocalTime = localTime }
}

func WithCompress(compress bool) Option {
	return func(c *Config) { c.Compress = compress }
}

func WithSensitiveKeys(SensitiveKeys []string) Option {
	return func(c *Config) { c.SensitiveKeys = SensitiveKeys }
}

func WithThreshold(lv zapcore.Level) Option {
	return func(c *Config) { c.Alert.Threshold = lv }
}

func WithMaxInterval(maxInterval time.Duration) Option {
	return func(c *Config) { c.Alert.MaxInterval = maxInterval }
}

func WithAlertQueueSize(queueSize int) Option {
	return func(c *Config) { c.Alert.QueueSize = queueSize }
}

func WithMaxBatchCnt(maxBatchCnt int) Option {
	return func(c *Config) { c.Alert.MaxBatchCnt = maxBatchCnt }
}

func WithMaxRetries(maxRetries int) Option {
	return func(c *Config) { c.Alert.RetryPolicy.MaxRetries = maxRetries }
}

func WithPrefix(prefix string) Option {
	return func(c *Config) { c.Alert.Prefix = prefix }
}

func WithRateLimiter(rate time.Duration, burst int) Option {
	return func(c *Config) {
		c.Alert.LimitPolicy.Limit = rate
		c.Alert.LimitPolicy.Burst = burst
	}
}

func WithToken(token string) Option {
	return func(c *Config) { c.Alert.Notification.Telegram.Token = token }
}

func WithChatID(chatID string) Option {
	return func(c *Config) { c.Alert.Notification.Telegram.ChatID = chatID }
}

type Config struct {
	Mode          Mode            // 日志模式：Development("dev") / Production("prod")
	Level         string          // 日志级别：debug/info/warn/error/dpanic/panic/fatal
	Directory     string          // 日志目录，开发模式默认"./logs"，生产建议"/var/log/app"
	Filename      string          // 普通日志文件名，默认"app.log"
	ErrorFilename string          // 错误日志文件名，默认"app_error.log"（存储error及以上级别日志）
	MaxSize       int             // 单个日志文件最大大小(MB)，默认200
	MaxAge        int             // 日志保留天数，默认7
	MaxBackups    int             // 保留的旧日志文件数量，默认10
	Compress      bool            // 是否压缩旧日志，默认true
	LocalTime     bool            // 是否使用本地时间命名日志，默认true (false 则使用 UTC 时间）
	QueueSize     int             // 异步日志队列大小，默认2048
	SensitiveKeys []string        //
	Sampling      *SamplingConfig //
	Alert         Alert           // 日志告警配置
}
type SamplingConfig struct {
	Enabled    bool
	Initial    int
	Thereafter int
	Window     time.Duration
}
type Alert struct {
	Threshold    zapcore.Level
	MaxInterval  time.Duration
	QueueSize    int
	MaxBatchCnt  int
	Prefix       string
	RetryPolicy  RetryPolicy
	LimitPolicy  LimitPolicy
	Notification Notification
}

type RetryPolicy struct {
	MaxRetries  int
	Backoff     time.Duration
	MinInterval time.Duration
}
type LimitPolicy struct {
	Limit time.Duration
	Burst int
}

type Notification struct {
	Telegram TelegramConfig
}
type TelegramConfig struct {
	Token  string
	ChatID string
}

func defaultConfig() *Config {
	cfg := &Config{
		Mode:          Development,
		Level:         "debug",
		Directory:     "./logs",
		Filename:      "app.log",
		ErrorFilename: "error.log",
		MaxSize:       200,
		MaxAge:        7,
		MaxBackups:    10,
		Compress:      true,
		LocalTime:     true,
		QueueSize:     4096,
		SensitiveKeys: []string{},
		Sampling: &SamplingConfig{
			Enabled:    true,
			Window:     time.Second,
			Initial:    100,
			Thereafter: 100,
		},
		Alert: Alert{
			Threshold:   zapcore.ErrorLevel,
			MaxInterval: 3 * time.Second,
			QueueSize:   4096,
			MaxBatchCnt: 10,
			Prefix:      "",
			LimitPolicy: LimitPolicy{
				Limit: 300 * time.Millisecond,
				Burst: 1,
			},
			RetryPolicy: RetryPolicy{
				MaxRetries:  1,
				Backoff:     200 * time.Millisecond,
				MinInterval: 500 * time.Millisecond,
			},
		},
	}
	return cfg
}

// validate.go
func (c *Config) validate() error {
	// 验证日志级别
	validLevels := map[string]bool{
		"debug":  true,
		"info":   true,
		"warn":   true,
		"error":  true,
		"dpanic": true,
		"panic":  true,
		"fatal":  true,
	}
	if !validLevels[c.Level] {
		return fmt.Errorf("invalid log level: %s", c.Level)
	}

	// 验证模式
	if c.Mode != Development && c.Mode != Production {
		return fmt.Errorf("invalid mode: %s", c.Mode)
	}

	// 验证目录路径
	if c.Directory == "" {
		return errors.New("log directory cannot be empty")
	}

	//// 路径校验
	//if !filepath.IsAbs(c.Directory) {
	//	return errors.New("directory must be absolute path")
	//}

	// 验证文件名
	if c.Filename == "" {
		return errors.New("log filename cannot be empty")
	}

	// 文件名校验
	if containsInvalidChars(c.Filename) {
		return errors.New("invalid filename")
	}

	// 验证错误日志文件名
	if c.ErrorFilename == "" {
		return errors.New("error log filename cannot be empty")
	}

	// 验证文件大小限制
	if c.MaxSize <= 0 {
		return errors.New("max size must be positive")
	}

	// 验证保留天数
	if c.MaxAge <= 0 {
		return errors.New("max age must be positive")
	}

	// 验证备份数量
	if c.MaxBackups < 0 {
		return errors.New("max backups cannot be negative")
	}

	// 验证队列大小
	if c.QueueSize <= 0 {
		return errors.New("queue size must be positive")
	}

	// 采样配置校验
	if c.Sampling != nil && c.Sampling.Enabled {
		if c.Sampling.Window < time.Second {
			return errors.New("sampling window too small")
		}
	}

	// 验证告警配置
	if err := c.Alert.validate(); err != nil {
		return fmt.Errorf("alert config error: %w", err)
	}

	// 敏感词去重
	c.SensitiveKeys = uniqueStrings(c.SensitiveKeys)

	return nil
}

func (a *Alert) validate() error {
	// 验证告警队列大小
	if a.QueueSize <= 0 {
		return errors.New("alert queue size must be positive")
	}

	// 验证最大批量数
	if a.MaxBatchCnt <= 0 {
		return errors.New("max batch count must be positive")
	}

	// 验证最大重试次数
	if a.RetryPolicy.MaxRetries <= 0 {
		return errors.New("max retries cannot be negative")
	}

	// 验证发送间隔
	if a.MaxInterval <= 0 {
		return errors.New("max interval must be positive")
	}

	// 验证限流速率
	if a.LimitPolicy.Limit <= 0 || a.LimitPolicy.Burst <= 0 {
		return errors.New("limiter rate must be positive")
	}

	// 验证Telegram配置
	if a.Notification.Telegram.Token != "" || a.Notification.Telegram.ChatID != "" {
		if a.Notification.Telegram.Token == "" || a.Notification.Telegram.ChatID == "" {
			return errors.New("both telegram token and chat id must be provided")
		}
	}

	return nil
}

// 辅助函数
func containsInvalidChars(s string) bool {
	return strings.ContainsAny(s, `<>:"/\|?*`)
}

func uniqueStrings(slice []string) []string {
	keys := make(map[string]bool)
	list := []string{}
	for _, entry := range slice {
		if _, value := keys[entry]; !value {
			keys[entry] = true
			list = append(list, entry)
		}
	}
	return list
}
