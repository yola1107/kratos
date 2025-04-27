package event

import (
	"sync"

	"github.com/yola1107/kratos/v2/log"
)

// Handler EventHandler 事件回调函数
type Handler func(val any)

// Bus EventBus 实现事件总线
type Bus struct {
	mu          sync.RWMutex
	subscribers map[string][]Handler
}

func NewEventBus() *Bus {
	return &Bus{
		subscribers: make(map[string][]Handler),
	}
}

func (bus *Bus) Subscribe(key string, fn Handler) {
	if key == "" || fn == nil {
		log.Warnf("key is empty or handler is nil")
		return
	}
	bus.mu.Lock()
	defer bus.mu.Unlock()
	bus.subscribers[key] = append(bus.subscribers[key], fn)
}

func (bus *Bus) Publish(key string, val any) {
	bus.mu.RLock()
	// 复制订阅者列表避免阻塞发布
	subs := make([]Handler, len(bus.subscribers[key]))
	copy(subs, bus.subscribers[key])
	bus.mu.RUnlock()

	// 异步执行订阅者回调
	for _, fn := range subs {
		go func(f Handler) {
			defer func() {
				if r := recover(); r != nil {
					log.Errorf("event handler panic: key=%q, error=%v", key, r)
				}
			}()
			f(val)
		}(fn)
	}
}
