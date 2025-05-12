package zap

import (
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/yola1107/kratos/v2/log"
)

type TelegramSender struct {
	Token  string
	ChatID string
	client *http.Client
}

func NewTelegramSender(config TelegramConfig) (*TelegramSender, error) {
	if config.Token == "" || config.ChatID == "" {
		return nil, fmt.Errorf("telegram Token or ChatID is empty")
	}

	return &TelegramSender{
		Token:  config.Token,
		ChatID: config.ChatID,
		client: &http.Client{
			Timeout: 1 * time.Second,
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
		content += msg
		content += "\n\n" // 用两个换行分隔多条消息
	}
	content += "\n\n---------\n\n"

	//fmt.Printf("=========>%+v send %d \n", time.Now().Format("2006-01-02 15:04:05.000"), len(messages))
	//fmt.Printf("=========>%+v send %d content: \n%+v", time.Now().Format("2006-01-02 15:04:05.000"), len(messages), content)
	//return nil

	_, err := t.client.PostForm(
		"https://api.telegram.org/bot"+t.Token+"/sendMessage",
		url.Values{
			"chat_id": {t.ChatID},
			"text":    {content},
		},
	)
	if err != nil {
		//log.Warnf(" %v cnt=%d \n%+v", err, len(messages), messages)
		fmt.Printf("%+v %v cnt=%d \n", time.Now().Format("2006-01-02 15:04:05.000"), err, len(messages))
	}
	return err
}

func (t *TelegramSender) Close() error {
	t.client.CloseIdleConnections()
	log.Infof("telegram closed.")
	return nil
}
