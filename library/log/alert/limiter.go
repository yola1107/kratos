package alert

import (
	"context"
	"sync"

	"golang.org/x/time/rate"

	"github.com/yola1107/kratos/v2/library/log/config"
)

// RateLimiter 速率限制器
type RateLimiter struct {
	config   config.RateLimit
	limiter  *rate.Limiter
	configMu sync.RWMutex
}

// NewRateLimiter 创建新的速率限制器
func NewRateLimiter(config config.RateLimit) *RateLimiter {
	if !config.Enabled {
		// 无限制情况
		return &RateLimiter{
			config:  config,
			limiter: rate.NewLimiter(rate.Inf, 0),
		}
	}
	return &RateLimiter{
		limiter: rate.NewLimiter(rate.Every(config.Interval), config.Burst),
		config:  config,
	}
}

// Allow 检查是否允许通过(非阻塞)
func (r *RateLimiter) Allow() bool {
	r.configMu.RLock()
	defer r.configMu.RUnlock()
	return r.limiter.Allow()
}

// Wait 等待直到允许通过(阻塞)
func (r *RateLimiter) Wait(ctx context.Context) error {
	r.configMu.RLock()
	defer r.configMu.RUnlock()
	return r.limiter.Wait(ctx)
}

//// UpdateConfig 动态更新配置
//func (r *RateLimiter) UpdateConfig(newConfig RateLimitConfig) {
//	r.configMu.Lock()
//	defer r.configMu.Unlock()
//
//	if !newConfig.Enabled {
//		r.limiter.SetLimit(rate.Inf)
//		r.limiter.SetBurst(0)
//	} else if newConfig.Interval > 0 {
//		r.limiter.SetLimit(rate.Every(newConfig.Interval))
//		r.limiter.SetBurst(newConfig.Burst)
//	}
//	r.config = newConfig
//}
//
//// CurrentConfig 获取当前配置
//func (r *RateLimiter) CurrentConfig() RateLimitConfig {
//	r.configMu.RLock()
//	defer r.configMu.RUnlock()
//	return r.config
//}
