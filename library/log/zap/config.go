package zap

import (
	"time"

	"github.com/yola1107/kratos/v2/log"
	"go.uber.org/zap/zapcore"
)

type Mode string

const (
	Development Mode = "dev"
	Production  Mode = "prod"
)

type Option func(*Config)

func WithMode(m Mode) Option {
	return func(c *Config) {
		switch m {
		case Development, Production:
			c.Mode = m
		default:
			log.Warnf("Unknow Logger Mode(%v)", m)
		}
	}
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
	Mode          Mode          // 日志模式：Development("dev") / Production("prod")
	Level         string        // 日志级别：debug/info/warn/error/dpanic/panic/fatal
	Directory     string        // 日志目录，开发模式默认"./logs"，生产建议"/var/log/app"
	Filename      string        // 普通日志文件名，默认"app.log"
	ErrorFilename string        // 错误日志文件名，默认"app_error.log"（存储error及以上级别日志）
	MaxSize       int           // 单个日志文件最大大小(MB)，默认200
	MaxAge        int           // 日志保留天数，默认7
	MaxBackups    int           // 保留的旧日志文件数量，默认10
	FlushInterval time.Duration // 日志刷盘间隔，默认3s
	Compress      bool          // 是否压缩旧日志，默认true
	QueueSize     int           // 异步日志队列大小，默认2048
	PoolSize      int           // 内存对象池大小，默认512
	LocalTime     bool          // 是否使用本地时间命名日志，默认true (false 则使用 UTC 时间）
	Alert         Alert         // 日志告警配置
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

func DefaultConfig(opts ...Option) *Config {
	cfg := &Config{
		Mode:          Development,
		Level:         "debug",         // 开发环境更详细日志
		Directory:     "./logs",        // "./logs"
		Filename:      "app.log",       // "app.log",
		ErrorFilename: "app_error.log", // "app_error.log",
		MaxSize:       200,             // 单个日志文件最大200MB
		MaxAge:        7,               // 保留7天
		MaxBackups:    10,              // 保留10个备份
		FlushInterval: 3 * time.Second, // 刷新间隔
		Compress:      true,            // 启用压缩
		LocalTime:     true,            // 本地时间命名日志
		QueueSize:     2048,            // 增大队列缓冲
		PoolSize:      512,             // 更大的对象池
		//SensitiveKeys: []string{"password", "token", "secret"},
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
	for _, opt := range opts {
		opt(cfg)
	}
	return cfg
}
