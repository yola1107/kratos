package websocket

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestClientCreation(t *testing.T) {
	// 测试基本的客户端创建（虽然会因为无法连接而失败，但可以测试配置逻辑）
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // 立即取消，避免实际连接

	// 这个测试主要是验证配置选项是否正确设置
	// 实际的连接测试需要 mock 或真实的 websocket 服务器
	_, err := NewClient(ctx, WithEndpoint("ws://test:8080"))
	assert.Error(t, err) // 应该失败，因为上下文已取消
}

func TestSessionManager(t *testing.T) {
	mgr := NewSessionManager()
	assert.Equal(t, int32(0), mgr.Len())

	// 创建一个 mock session
	session := &Session{
		id: "test-session",
	}

	// 测试添加会话
	mgr.Add(session)
	assert.Equal(t, int32(1), mgr.Len())

	// 测试获取会话
	retrieved := mgr.Get("test-session")
	assert.NotNil(t, retrieved)
	assert.Equal(t, "test-session", retrieved.ID())

	// 测试删除会话
	mgr.Delete(session)
	assert.Equal(t, int32(0), mgr.Len())
}

func TestCloseReason(t *testing.T) {
	tests := []struct {
		name     string
		force    bool
		msg      []string
		expected string
	}{
		{"normal close", false, nil, "Normal Close"},
		{"force close", true, nil, "Force Close"},
		{"with message", false, []string{"test"}, "Normal Close: test"},
		{"force with message", true, []string{"error"}, "Force Close: error"},
		{"multiple messages", false, []string{"msg1", "msg2"}, "Normal Close: msg1; msg2"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := closeReason(nil, tt.force, tt.msg...)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestCalculateBackoff(t *testing.T) {
	client := &Client{
		opts: &clientOptions{
			retryDelay: 1 * time.Second,
		},
	}

	// 测试不同的尝试次数
	tests := []struct {
		attempt int32
		minTime time.Duration
		maxTime time.Duration
	}{
		{1, 900 * time.Millisecond, 1100 * time.Millisecond},  // delay * 0.9 to delay * 1.1
		{2, 1350 * time.Millisecond, 1650 * time.Millisecond}, // delay * 1.5 * 0.9 to delay * 1.5 * 1.1
	}

	for _, tt := range tests {
		delay := client.calculateBackoff(tt.attempt)
		assert.True(t, delay >= tt.minTime, "delay should be >= minTime")
		assert.True(t, delay <= tt.maxTime, "delay should be <= maxTime")
	}
}
