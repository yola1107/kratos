package conf

import (
	"os"
)

const (
	TelegramTokenKey  = "LOG_TG_TOKEN"
	TelegramChatIDKey = "LOG_TG_CHAT_ID"
)

func DefaultConfig(opts ...Option) *Bootstrap {
	c := &Log{
		Logger: &Logger{
			Mode:       MODE_DEV,
			AppName:    "app", // app,
			Level:      "debug",
			Directory:  "./logs",
			FormatJson: false,
			ErrorFile:  false,
			Sensitive:  []string{},
			Rotate: &Rotate{
				MaxSizeMB:  100,
				MaxBackups: 7,
				MaxAgeDays: 7,
				Compress:   true,
				LocalTime:  true,
			},
		},
		Alerter: &Alerter{
			Enabled: false,
			Prefix:  "",     // "<" + app + ">",
			Format:  "html", // json/html
		},
		Telegram: &Telegram{
			Token:  os.Getenv(TelegramTokenKey),
			ChatID: os.Getenv(TelegramChatIDKey),
		},
	}

	for _, opt := range opts {
		opt(c)
	}

	return &Bootstrap{
		Log: c,
	}
}

type Option func(*Log)

// Logger Options

func WithAppName(appName string) Option {
	return func(c *Log) {
		c.Logger.AppName = appName
		c.Alerter.Prefix = appName
	}
}

func WithProduction() Option {
	return func(c *Log) {
		c.Logger.Mode = MODE_PROD
		c.Logger.Level = "info"
	}
}

func WithLevel(level string) Option {
	return func(c *Log) { c.Logger.Level = level }
}

func WithDirectory(dir string) Option {
	return func(c *Log) { c.Logger.Directory = dir }
}

func WithFormatJson(enabled bool) Option {
	return func(c *Log) { c.Logger.FormatJson = enabled }
}

func WithErrorFile(enabled bool) Option {
	return func(c *Log) { c.Logger.ErrorFile = enabled }
}

func WithSensitive(keys []string) Option {
	return func(c *Log) { c.Logger.Sensitive = keys }
}

func WithMaxSizeMB(size int32) Option {
	return func(c *Log) { c.Logger.Rotate.MaxSizeMB = size }
}

func WithMaxAgeDays(days int32) Option {
	return func(c *Log) { c.Logger.Rotate.MaxAgeDays = days }
}

func WithMaxBackups(count int32) Option {
	return func(c *Log) { c.Logger.Rotate.MaxBackups = count }
}

func WithCompress(compress bool) Option {
	return func(c *Log) { c.Logger.Rotate.Compress = compress }
}

func WithLocalTime(local bool) Option {
	return func(c *Log) { c.Logger.Rotate.LocalTime = local }
}

// Alerter Options

func WithAlertEnable(enable bool) Option {
	return func(c *Log) { c.Alerter.Enabled = enable }
}

func WithAlertPrefix(prefix string) Option {
	return func(c *Log) { c.Alerter.Prefix = prefix }
}

func WithAlertFormat(format string) Option {
	return func(c *Log) { c.Alerter.Format = format }
}

// Telegram Options

func WithTelegramToken(token string) Option {
	return func(c *Log) { c.Telegram.Token = token }
}

func WithTelegramChatID(chatID string) Option {
	return func(c *Log) { c.Telegram.ChatID = chatID }
}
