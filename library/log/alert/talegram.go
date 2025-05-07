package alert

import (
	"fmt"
	"net/http"
	"time"

	"github.com/yola1107/kratos/v2/library/log/config"
	"github.com/yola1107/kratos/v2/log"
)

type TelegramSender struct {
	config config.Telegram
	client *http.Client
}

func NewTelegramSender(config config.Telegram) (*TelegramSender, error) {
	if !config.Enabled {
		return nil, fmt.Errorf("telegram sender is disabled")
	}
	if config.Token == "" || config.ChatID == "" {
		return nil, fmt.Errorf("token or ChatID is empty")
	}
	return &TelegramSender{
		config: config,
		client: &http.Client{
			Timeout: 5 * time.Second,
			Transport: &http.Transport{
				MaxIdleConns:       10,
				IdleConnTimeout:    10 * time.Second,
				DisableCompression: true,
			}},
	}, nil
}

func (t *TelegramSender) Send(messages []string) error {
	content := ""
	for _, msg := range messages {
		content += msg // 用两个换行分隔多条消息
	}
	content += "\n\n---------\n\n"

	fmt.Printf("=========>%+v send content: \n%+v", time.Now().Format("2006-01-02 15:04:05.000"), content)
	return nil

	//_, err := t.client.PostForm(
	//	"https://api.telegram.org/bot"+t.config.Token+"/sendMessage",
	//	url.Values{
	//		"chat_id": {t.config.ChatID},
	//		"text":    {"content"},
	//	},
	//)
	//if err != nil {
	//	fmt.Printf(" %v\n", err)
	//}
	//return err
}

func (t *TelegramSender) Close() error {
	t.client.CloseIdleConnections()
	log.Infof("telegram closed")
	return nil
}
