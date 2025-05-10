package zap

import (
	"errors"
	"fmt"
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
	return func(c *Config) { c.Alert.MaxRetries = maxRetries }
}

func WithPrefix(prefix string) Option {
	return func(c *Config) { c.Alert.Prefix = prefix }
}

func WithRateLimiter(rate time.Duration) Option {
	return func(c *Config) { c.Alert.Limiter = rate }
}

func WithToken(token string) Option {
	return func(c *Config) { c.Alert.Telegram.Token = token }
}

func WithChatID(chatID string) Option {
	return func(c *Config) { c.Alert.Telegram.ChatID = chatID }
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
	AsyncConsole  bool            //
	Sampling      *SamplingConfig //
	Alert         Alert           // 日志告警配置
}
type SamplingConfig struct {
	Initial    int
	Thereafter int
}
type Alert struct {
	Threshold   zapcore.Level // 触发日志级别
	MaxInterval time.Duration // 发送间隔
	QueueSize   int           // 队列大小
	MaxBatchCnt int           // 最大批量数
	MaxRetries  int           // 最大重试
	Prefix      string        // 消息前缀
	Limiter     time.Duration // 限流速率
	Telegram    Telegram
}

type Telegram struct {
	Token  string
	ChatID string
}

func defaultConfig() *Config {
	cfg := &Config{
		Mode:          Development,
		Level:         "debug",         // 开发环境更详细日志
		Directory:     "./logs",        // "./logs"
		Filename:      "app.log",       // "app.log",
		ErrorFilename: "app_error.log", // "app_error.log",
		MaxSize:       200,             // 单个日志文件最大200MB
		MaxAge:        7,               // 保留7天
		MaxBackups:    10,              // 保留10个备份
		Compress:      true,            // 启用压缩
		LocalTime:     true,            // 本地时间命名日志
		QueueSize:     4096,            // 增大队列缓冲
		AsyncConsole:  true,
		//SensitiveKeys: []string{"password", "token", "secret"},
		Sampling: &SamplingConfig{
			Initial:    100,
			Thereafter: 100,
		},
		Alert: Alert{
			Threshold:   zapcore.ErrorLevel,
			MaxInterval: 3 * time.Second,
			QueueSize:   2048,
			MaxBatchCnt: 10,
			MaxRetries:  1,
			Prefix:      "",
			Limiter:     300 * time.Millisecond,
			Telegram:    Telegram{},
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

	// 验证文件名
	if c.Filename == "" {
		return errors.New("log filename cannot be empty")
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

	// 验证告警配置
	if err := c.Alert.validate(); err != nil {
		return fmt.Errorf("alert config error: %w", err)
	}

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
	if a.MaxRetries < 0 {
		return errors.New("max retries cannot be negative")
	}

	// 验证发送间隔
	if a.MaxInterval <= 0 {
		return errors.New("max interval must be positive")
	}

	// 验证限流速率
	if a.Limiter <= 0 {
		return errors.New("limiter rate must be positive")
	}

	// 验证Telegram配置
	if a.Telegram.Token != "" || a.Telegram.ChatID != "" {
		if a.Telegram.Token == "" || a.Telegram.ChatID == "" {
			return errors.New("both telegram token and chat id must be provided")
		}
	}

	return nil
}
