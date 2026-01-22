package redis

import (
	"fmt"
	"net"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

// 默认配置值
const (
	defaultHost        = "127.0.0.1"
	defaultPort        = 6379
	defaultMinIdle     = 5
	defaultMaxIdle     = 10
	defaultPoolSize    = 10
	defaultMaxLifetime = 2 * time.Minute
	defaultMaxIdleTime = 5 * time.Minute
)

// ClientOption 配置函数类型
type ClientOption func(*redis.UniversalOptions)

// NewClient 创建Redis客户端，可接受多个配置选项
func NewClient(opts ...ClientOption) redis.UniversalClient {
	// 初始化默认配置
	options := &redis.UniversalOptions{
		Addrs:           []string{fmt.Sprintf("%s:%d", defaultHost, defaultPort)},
		Password:        "",
		DB:              0,
		PoolSize:        defaultPoolSize,
		MinIdleConns:    defaultMinIdle,
		MaxIdleConns:    defaultMaxIdle,
		ConnMaxLifetime: defaultMaxLifetime,
		ConnMaxIdleTime: defaultMaxIdleTime,
	}

	// 应用所有配置选项
	for _, opt := range opts {
		opt(options)
	}

	return redis.NewUniversalClient(options)
}

// WithAddrs 设置Redis多个地址（用于集群或哨兵）
func WithAddrs(addrs ...string) ClientOption {
	return func(o *redis.UniversalOptions) {
		if len(addrs) > 0 {
			o.Addrs = addrs
		}
	}
}

// WithAddress 设置Redis完整地址（兼容单地址设置，操作第一个地址）
func WithAddress(addr string) ClientOption {
	return func(o *redis.UniversalOptions) {
		// 简单的地址格式验证
		if _, _, err := net.SplitHostPort(addr); err == nil {
			if len(o.Addrs) == 0 {
				o.Addrs = []string{addr}
			} else {
				o.Addrs[0] = addr
			}
		}
	}
}

// WithHost 设置Redis主机地址（操作第一个地址）
func WithHost(host string) ClientOption {
	return func(o *redis.UniversalOptions) {
		addr := fmt.Sprintf("%s:%d", defaultHost, defaultPort)
		if len(o.Addrs) > 0 {
			addr = o.Addrs[0]
		}
		// 从现有地址中提取端口
		_, port, err := net.SplitHostPort(addr)
		if err != nil {
			// 如果现有地址无效，使用默认端口
			port = strconv.Itoa(defaultPort)
		}
		newAddr := net.JoinHostPort(host, port)
		if len(o.Addrs) == 0 {
			o.Addrs = []string{newAddr}
		} else {
			o.Addrs[0] = newAddr
		}
	}
}

// WithPort 设置Redis端口（操作第一个地址）
func WithPort(port int) ClientOption {
	return func(o *redis.UniversalOptions) {
		addr := fmt.Sprintf("%s:%d", defaultHost, defaultPort)
		if len(o.Addrs) > 0 {
			addr = o.Addrs[0]
		}
		// 从现有地址中提取主机
		host, _, err := net.SplitHostPort(addr)
		if err != nil {
			// 如果现有地址无效，使用默认主机
			host = defaultHost
		}
		newAddr := net.JoinHostPort(host, strconv.Itoa(port))
		if len(o.Addrs) == 0 {
			o.Addrs = []string{newAddr}
		} else {
			o.Addrs[0] = newAddr
		}
	}
}

// WithPassword 设置Redis密码
func WithPassword(pass string) ClientOption {
	return func(o *redis.UniversalOptions) {
		o.Password = pass
	}
}

// WithDB 选择Redis数据库
func WithDB(db int) ClientOption {
	return func(o *redis.UniversalOptions) {
		if db >= 0 {
			o.DB = db
		}
	}
}

// WithPoolSize 设置连接池大小
func WithPoolSize(size int) ClientOption {
	return func(o *redis.UniversalOptions) {
		if size > 0 {
			o.PoolSize = size
		}
	}
}

// WithMinIdleConn 设置最小空闲连接数
func WithMinIdleConn(n int) ClientOption {
	return func(o *redis.UniversalOptions) {
		if n >= 0 {
			o.MinIdleConns = n
		}
	}
}

// WithMaxIdleConn 设置最大空闲连接数
func WithMaxIdleConn(n int) ClientOption {
	return func(o *redis.UniversalOptions) {
		if n >= 0 {
			o.MaxIdleConns = n
		}
	}
}

// WithConnMaxLifetime 设置连接最大生存时间
func WithConnMaxLifetime(d time.Duration) ClientOption {
	return func(o *redis.UniversalOptions) {
		if d > 0 {
			o.ConnMaxLifetime = d
		}
	}
}

// WithConnMaxIdleTime 设置连接最大空闲时间
func WithConnMaxIdleTime(d time.Duration) ClientOption {
	return func(o *redis.UniversalOptions) {
		if d > 0 {
			o.ConnMaxIdleTime = d
		}
	}
}

// 示例使用：
// client := redis.NewClient(
//     redis.WithAddrs("127.0.0.1:6379", "127.0.0.1:6380"),
//     redis.WithPassword("securepassword"),
//     redis.WithDB(0),
//     redis.WithPoolSize(20),
//     redis.WithMinIdleConn(5),
//     redis.WithMaxIdleConn(10),
//     redis.WithConnMaxLifetime(10*time.Minute),
//     redis.WithConnMaxIdleTime(30*time.Minute),
// )
