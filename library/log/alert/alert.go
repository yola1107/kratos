package alert

import (
	"time"

	"go.uber.org/zap/zapcore"
)

//
//// AlertCore 定义告警功能的接口
//type AlertCore interface {
//	zapcore.Core
//	io.Closer
//}
//
//type alertCort struct {
//	zapcore.LevelEnabler
//	enc zapcore.Encoder
//	//out zapcore.WriteSyncer
//	senders []Sender
//}
//
//func New(out zapcore.WriteSyncer) (AlertCore, error) {
//	return nil, nil
//}
//
//func (c *alertCort) Write(ent zapcore.Entry, fields []zapcore.Field) error {
//	buf, err := c.enc.EncodeEntry(ent, fields)
//	if err != nil {
//		return err
//	}
//	_, err = c.out.Write(buf.Bytes())
//	buf.Free()
//	return err
//}
//
//func (c *alertCort) Sync() error {
//	return c.out.Sync()
//}
//
//func (c *alertCort) Close() error {
//	return c.Sync()
//}
//
//func (c *alertCort) With(fields []zapcore.Field) zapcore.Core {
//	clone := *c
//	clone.enc = clone.enc.Clone()
//	for i := range fields {
//		fields[i].AddTo(clone.enc)
//	}
//	return &clone
//}
//
//func (c *alertCort) Check(ent zapcore.Entry, ce *zapcore.CheckedEntry) *zapcore.CheckedEntry {
//	if c.Enabled(ent.Level) {
//		return ce.AddCore(ent, c)
//	}
//	return ce
//}

//

// alert_core.go
//package alert
//
//type Sender interface {
//	Send(entry zapcore.Entry, fields []zap.Field) error
//	Name() string
//	io.Closer
//}
//
//// level_enabler.go - 级别过滤器
//type LevelEnabler interface {
//	Enabled(level zapcore.Level) bool
//}
//
//type AlertCore struct {
//	fields     []zap.Field
//	queue      chan alertTask
//	workerSize int
//	wg         sync.WaitGroup
//	closeOnce  sync.Once
//
//	senders  []Sender
//	enablers []LevelEnabler
//}
//
//type alertTask struct {
//	entry  zapcore.Entry
//	fields []zap.Field
//}
//
//func NewAlertCore(workerSize int, bufferSize int) *AlertCore {
//	ac := &AlertCore{
//		queue:      make(chan alertTask, bufferSize),
//		workerSize: workerSize,
//	}
//	ac.startWorkers()
//	return ac
//}
//
//func (ac *AlertCore) AddSender(sender Sender) {
//	ac.senders = append(ac.senders, sender)
//}
//
//func (ac *AlertCore) AddLevelEnabler(enabler LevelEnabler) {
//	ac.enablers = append(ac.enablers, enabler)
//}
//
//func (ac *AlertCore) startWorkers() {
//	for i := 0; i < ac.workerSize; i++ {
//		ac.wg.Add(1)
//		go ac.worker()
//	}
//}
//
//func (ac *AlertCore) worker() {
//	defer ac.wg.Done()
//	for task := range ac.queue {
//		ac.processAlert(task.entry, task.fields)
//	}
//}
//
//func (ac *AlertCore) processAlert(entry zapcore.Entry, fields []zap.Field) {
//	// 合并全局字段
//	allFields := make([]zap.Field, 0, len(fields)+len(ac.fields))
//	allFields = append(allFields, ac.fields...)
//	allFields = append(allFields, fields...)
//
//	// 检查是否应该发送告警
//	if !ac.shouldAlert(entry.Level) {
//		return
//	}
//
//	// 发送给所有sender
//	var errs []error
//	for _, sender := range ac.senders {
//		if err := sender.Send(entry, allFields); err != nil {
//			errs = append(errs, fmt.Errorf("%s sender error: %w", sender.Name(), err))
//		}
//	}
//
//	if len(errs) > 0 {
//		// 这里可以记录发送失败的错误
//	}
//}
//
//func (ac *AlertCore) shouldAlert(level zapcore.Level) bool {
//	if len(ac.enablers) == 0 {
//		return true
//	}
//
//	for _, enabler := range ac.enablers {
//		if enabler.Enabled(level) {
//			return true
//		}
//	}
//	return false
//}
//
//func (ac *AlertCore) Write(entry zapcore.Entry, fields []zapcore.Field) error {
//	// 转换为zap.Field类型
//	zapFields := make([]zap.Field, len(fields))
//	for i, f := range fields {
//		zapFields[i] = zap.Field(f)
//	}
//
//	select {
//	case ac.queue <- alertTask{entry: entry, fields: zapFields}:
//		return nil
//	default:
//		return errors.New("alert queue is full")
//	}
//}
//
//func (ac *AlertCore) Sync() error {
//	// 等待队列处理完成
//	close(ac.queue)
//	ac.wg.Wait()
//
//	var errs []error
//	for _, sender := range ac.senders {
//		if err := sender.Close(); err != nil {
//			errs = append(errs, err)
//		}
//	}
//	return errors.Join(errs...)
//}
//
//func (ac *AlertCore) Close() error {
//	return ac.Sync()
//}
//
//// 实现zapcore.Core接口的其他方法
//func (ac *AlertCore) Enabled(level zapcore.Level) bool {
//	return ac.shouldAlert(level)
//}
//
//func (ac *AlertCore) With(fields []zapcore.Field) zapcore.Core {
//	clone := *ac
//	clone.fields = make([]zap.Field, len(ac.fields)+len(fields))
//	copy(clone.fields, ac.fields)
//
//	for i, f := range fields {
//		clone.fields[len(ac.fields)+i] = zap.Field(f)
//	}
//
//	return &clone
//}
//
//func (ac *AlertCore) Check(ent zapcore.Entry, ce *zapcore.CheckedEntry) *zapcore.CheckedEntry {
//	if ac.Enabled(ent.Level) {
//		return ce.AddCore(ent, ac)
//	}
//	return ce
//}

//type Sender interface {
//	Name() string
//	Send(content string) error
//	Close() error
//}

//type AlertCore struct {
//	zapcore.Core // 组合原 zapcore.Core
//	fields       []zap.Field
//
//	senders []Sender // 所有 Sender 列表
//	wg    sync.WaitGroup
//}

// Sender 通知器接口
type Sender interface {
	Send(text string) error
	Close() error
}

type (
	Config struct {
		QueueSize   int            `yaml:"queue_size"`    // 队列大小
		RateLimit   time.Duration  `yaml:"rate_limit"`    // 发送间隔
		MaxBatchCnt int            `yaml:"max_batch_cnt"` // 最大批量数
		MaxRetries  int            `yaml:"max_retries"`   // 最大重试
		Telegram    TelegramConfig `yaml:"telegram"`
	}
	TelegramConfig struct {
		Enabled   bool          `yaml:"enabled"`   // 是否启用
		Threshold zapcore.Level `yaml:"threshold"` // 日志级别
		Token     string        `yaml:"token"`     // Bot Token
		ChatID    string        `yaml:"chat_id"`   // 聊天ID
		Prefix    string        `yaml:"prefix"`    // 消息前缀
	}
)

// AlertCore 报警核心
type AlertCore struct {
	zapcore.LevelEnabler
	enc      zapcore.Encoder
	fields   []zapcore.Field
	notifier Sender
}

// NewAlertCore 创建报警核心
func NewAlertCore(enabler zapcore.LevelEnabler, enc zapcore.Encoder, cfg *Config) *AlertCore {
	//n := NewTelegramNotifier(cfg)
	//if n == nil {
	//	return nil
	//}
	//return &AlertCore{
	//	LevelEnabler: enabler,
	//	enc:          enc,
	//	notifier:     n,
	//}
	return nil
}

// With 添加字段
func (c *AlertCore) With(fields []zapcore.Field) zapcore.Core {
	// 即使 fields 为空也返回新实例
	clone := &AlertCore{
		LevelEnabler: c.LevelEnabler,
		enc:          c.enc,
		notifier:     c.notifier,
	}

	// 完全拷贝所有字段
	clone.fields = make([]zapcore.Field, len(c.fields), len(c.fields)+len(fields))
	copy(clone.fields, c.fields)
	clone.fields = append(clone.fields, fields...)

	return clone
}

// Check 检查日志级别
func (c *AlertCore) Check(ent zapcore.Entry, ce *zapcore.CheckedEntry) *zapcore.CheckedEntry {
	if c.Enabled(ent.Level) {
		return ce.AddCore(ent, c)
	}
	return ce
}

// Write 写入日志
func (c *AlertCore) Write(ent zapcore.Entry, fields []zapcore.Field) error {
	entryBuf, err := c.enc.EncodeEntry(ent, append(c.fields, fields...))
	if err != nil {
		return err
	}
	return c.notifier.Send(entryBuf.String())
	//return c.notifier.Send(truncateMessage(entryBuf.String()))
}

// Sync 同步日志
func (c *AlertCore) Sync() error { return nil }

// Close 关闭
func (c *AlertCore) Close() error { return c.notifier.Close() }
