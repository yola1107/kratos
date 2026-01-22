package xorm

import (
	"fmt"
	"time"

	"xorm.io/xorm"
	"xorm.io/xorm/log"
	"xorm.io/xorm/names"
)

// 默认配置值
const (
	defaultMaxIdleConns    = 10
	defaultMaxOpenConns    = 100
	defaultConnMaxLifetime = 2 * time.Hour
	defaultConnMaxIdleTime = 30 * time.Minute
	defaultShowSQL         = false
)

// EngineOption 配置函数类型
type EngineOption func(*engineConfig)

// engineConfig XORM引擎配置结构体
// 用于存储创建XORM引擎所需的所有配置参数
type engineConfig struct {
	// dataSourceName 数据源名称（DSN），数据库连接字符串
	// MySQL示例: "user:password@tcp(localhost:3306)/dbname?charset=utf8mb4&parseTime=True&loc=Local"
	// PostgreSQL示例: "postgres://user:password@localhost/dbname?sslmode=disable"
	// SQLite示例: "test.db" 或 ":memory:"
	dataSourceName string

	// driverName 数据库驱动名称
	// 支持的值: "mysql", "postgres", "sqlite3", "mssql", "oracle" 等
	driverName string

	// maxIdleConns 连接池中最大空闲连接数
	// 默认值: 10
	maxIdleConns int

	// maxOpenConns 连接池中最大打开连接数（同时使用的最大连接数）
	// 默认值: 100
	maxOpenConns int

	// connMaxLifetime 连接的最大生存时间
	// 超过此时间的连接将被关闭并重新创建，用于避免长时间连接导致的网络问题
	// 默认值: 2小时
	connMaxLifetime time.Duration

	// connMaxIdleTime 连接的最大空闲时间
	// 空闲连接超过此时间将被关闭，用于释放未使用的连接资源
	// 默认值: 30分钟
	connMaxIdleTime time.Duration

	// showSQL 是否在日志中显示执行的SQL语句
	// 开启后会在日志中输出所有执行的SQL语句，便于调试
	// 默认值: false
	showSQL bool

	// logger 自定义日志记录器
	// 如果设置了自定义logger，将使用此logger记录日志，否则使用默认日志级别
	// 默认值: nil（使用logLevel）
	logger log.Logger

	// logLevel 日志级别
	// 可选值: LOG_DEBUG, LOG_INFO, LOG_WARNING, LOG_ERR, LOG_OFF
	// 默认值: LOG_INFO
	logLevel log.LogLevel

	// tableMapper 表名映射器类型
	// 用于将Go结构体名称映射到数据库表名
	// 可选值: "snake" (默认，将 UserInfo 映射为 user_info), "same" (保持原样), "gonic" (类似snake但处理缩写更好)
	// 默认值: "snake"
	tableMapper string

	// columnMapper 列名映射器类型
	// 用于将Go结构体字段名称映射到数据库列名
	// 可选值: "snake" (默认，将 UserName 映射为 user_name), "same" (保持原样), "gonic" (类似snake但处理缩写更好，如ID映射为id而非i_d)
	// 默认值: "snake"
	columnMapper string
}

// NewEngine 创建XORM引擎，可接受多个配置选项
func NewEngine(opts ...EngineOption) (*xorm.Engine, error) {
	// 初始化默认配置
	config := &engineConfig{
		maxIdleConns:    defaultMaxIdleConns,
		maxOpenConns:    defaultMaxOpenConns,
		connMaxLifetime: defaultConnMaxLifetime,
		connMaxIdleTime: defaultConnMaxIdleTime,
		showSQL:         defaultShowSQL,
		logLevel:        log.LOG_INFO,
		tableMapper:     "snake",
		columnMapper:    "snake",
	}

	// 应用所有配置选项
	for _, opt := range opts {
		opt(config)
	}

	// 验证必需配置
	if config.dataSourceName == "" {
		return nil, fmt.Errorf("data source name is required")
	}
	if config.driverName == "" {
		return nil, fmt.Errorf("driver name is required, use WithDriver() or convenience methods like WithMySQL()")
	}

	// 创建引擎
	engine, err := xorm.NewEngine(config.driverName, config.dataSourceName)
	if err != nil {
		return nil, fmt.Errorf("failed to create xorm engine: %w", err)
	}

	// 设置连接池配置
	engine.SetMaxIdleConns(config.maxIdleConns)
	engine.SetMaxOpenConns(config.maxOpenConns)
	engine.SetConnMaxLifetime(config.connMaxLifetime)
	engine.SetConnMaxIdleTime(config.connMaxIdleTime)
	engine.ShowSQL(config.showSQL)

	// 设置日志
	if config.logger != nil {
		engine.SetLogger(config.logger)
	} else {
		engine.SetLogLevel(config.logLevel)
	}

	// 设置表名和列名映射
	if config.tableMapper == "snake" {
		engine.SetTableMapper(names.SnakeMapper{})
	} else if config.tableMapper == "same" {
		engine.SetTableMapper(names.SameMapper{})
	} else if config.tableMapper == "gonic" {
		engine.SetTableMapper(names.GonicMapper{})
	} else if config.tableMapper != "" {
		// 如果提供了无效的映射器值，返回错误
		engine.Close()
		return nil, fmt.Errorf("invalid table mapper: %s, supported values: snake, same, gonic", config.tableMapper)
	}

	if config.columnMapper == "snake" {
		engine.SetColumnMapper(names.SnakeMapper{})
	} else if config.columnMapper == "same" {
		engine.SetColumnMapper(names.SameMapper{})
	} else if config.columnMapper == "gonic" {
		engine.SetColumnMapper(names.GonicMapper{})
	} else if config.columnMapper != "" {
		// 如果提供了无效的映射器值，返回错误
		engine.Close()
		return nil, fmt.Errorf("invalid column mapper: %s, supported values: snake, same, gonic", config.columnMapper)
	}

	// 测试连接
	if err := engine.Ping(); err != nil {
		engine.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return engine, nil
}

// WithDataSource 设置数据源名称（DSN）
// 例如: "user:password@tcp(localhost:3306)/dbname?charset=utf8mb4&parseTime=True&loc=Local"
func WithDataSource(dsn string) EngineOption {
	return func(c *engineConfig) {
		c.dataSourceName = dsn
	}
}

// WithDriver 设置数据库驱动名称
// 支持: "mysql", "postgres", "sqlite3", "mssql", "oracle" 等
func WithDriver(driver string) EngineOption {
	return func(c *engineConfig) {
		c.driverName = driver
	}
}

// WithMySQL 便捷方法：配置MySQL连接
// 示例: WithMySQL("user:password@tcp(localhost:3306)/dbname?charset=utf8mb4")
func WithMySQL(dsn string) EngineOption {
	return func(c *engineConfig) {
		c.driverName = "mysql"
		c.dataSourceName = dsn
	}
}

// WithPostgreSQL 便捷方法：配置PostgreSQL连接
// 示例: WithPostgreSQL("postgres://user:password@localhost/dbname?sslmode=disable")
func WithPostgreSQL(dsn string) EngineOption {
	return func(c *engineConfig) {
		c.driverName = "postgres"
		c.dataSourceName = dsn
	}
}

// WithSQLite 便捷方法：配置SQLite连接
// 示例: WithSQLite("test.db")
func WithSQLite(dsn string) EngineOption {
	return func(c *engineConfig) {
		c.driverName = "sqlite3"
		c.dataSourceName = dsn
	}
}

// WithMaxIdleConns 设置最大空闲连接数
func WithMaxIdleConns(n int) EngineOption {
	return func(c *engineConfig) {
		if n > 0 {
			c.maxIdleConns = n
		}
	}
}

// WithMaxOpenConns 设置最大打开连接数
func WithMaxOpenConns(n int) EngineOption {
	return func(c *engineConfig) {
		if n > 0 {
			c.maxOpenConns = n
		}
	}
}

// WithConnMaxLifetime 设置连接最大生存时间
func WithConnMaxLifetime(d time.Duration) EngineOption {
	return func(c *engineConfig) {
		if d > 0 {
			c.connMaxLifetime = d
		}
	}
}

// WithConnMaxIdleTime 设置连接最大空闲时间
func WithConnMaxIdleTime(d time.Duration) EngineOption {
	return func(c *engineConfig) {
		if d > 0 {
			c.connMaxIdleTime = d
		}
	}
}

// WithShowSQL 设置是否显示SQL语句
func WithShowSQL(show bool) EngineOption {
	return func(c *engineConfig) {
		c.showSQL = show
	}
}

// WithLogger 设置自定义日志记录器
func WithLogger(logger log.Logger) EngineOption {
	return func(c *engineConfig) {
		c.logger = logger
	}
}

// WithLogLevel 设置日志级别
// 可选值: LOG_DEBUG, LOG_INFO, LOG_WARNING, LOG_ERR, LOG_OFF
func WithLogLevel(level log.LogLevel) EngineOption {
	return func(c *engineConfig) {
		c.logLevel = level
	}
}

// WithTableMapper 设置表名映射器
// 可选值: "snake" (默认), "same", "gonic"
func WithTableMapper(mapper string) EngineOption {
	return func(c *engineConfig) {
		c.tableMapper = mapper
	}
}

// WithColumnMapper 设置列名映射器
// 可选值: "snake" (默认), "same", "gonic"
func WithColumnMapper(mapper string) EngineOption {
	return func(c *engineConfig) {
		c.columnMapper = mapper
	}
}

// 示例使用：
// engine, err := xorm.NewEngine(
//     xorm.WithMySQL("user:password@tcp(localhost:3306)/dbname?charset=utf8mb4"),
//     xorm.WithMaxIdleConns(10),
//     xorm.WithMaxOpenConns(100),
//     xorm.WithConnMaxLifetime(2*time.Hour),
//     xorm.WithConnMaxIdleTime(30*time.Minute),
//     xorm.WithShowSQL(true),
//     xorm.WithLogLevel(log.LOG_INFO),
// )
// if err != nil {
//     log.Fatal(err)
// }
// defer engine.Close()
