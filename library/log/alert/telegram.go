package alert

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type TelegramOption func(*TelegramSender)

func WithTelegramTimeout(timeout time.Duration) TelegramOption {
	return func(s *TelegramSender) {
		s.timeout = timeout
	}
}

func WithTelegramFormatter(f func(zapcore.Entry, []zap.Field) string) TelegramOption {
	return func(s *TelegramSender) {
		s.formatter = f
	}
}

type TelegramSender struct {
	client    *http.Client
	token     string
	chatID    string
	timeout   time.Duration
	formatter func(zapcore.Entry, []zap.Field) string
}

func New(token, chatID string, opts ...TelegramOption) *TelegramSender {
	s := &TelegramSender{
		token:   token,
		chatID:  chatID,
		timeout: 5 * time.Second,
		formatter: func(ent zapcore.Entry, fields []zap.Field) string {
			var sb strings.Builder
			fmt.Fprintf(&sb, "[%s] %s\n", ent.Level.CapitalString(), ent.Message)
			for _, f := range fields {
				fmt.Fprintf(&sb, "- %s: %v\n", f.Key, f.Interface)
			}
			if ent.Stack != "" {
				sb.WriteString("\nStack Trace:\n")
				sb.WriteString(ent.Stack)
			}
			return sb.String()
		},
	}

	for _, opt := range opts {
		opt(s)
	}

	s.client = &http.Client{Timeout: s.timeout}
	return s
}

func (s *TelegramSender) Send(entry zapcore.Entry, fields []zap.Field) error {
	msg := s.formatter(entry, fields)
	apiURL := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", s.token)

	resp, err := s.client.PostForm(apiURL, url.Values{
		"chat_id": {s.chatID},
		"text":    {msg},
	})
	if err != nil {
		return fmt.Errorf("telegram API请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("telegram API返回错误状态码: %d", resp.StatusCode)
	}

	return nil
}

func (s *TelegramSender) Name() string {
	return "telegram"
}

func (s *TelegramSender) Close() error {
	// Telegram sender不需要特殊关闭逻辑
	return nil
}
