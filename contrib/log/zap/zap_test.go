package zap

import (
	"fmt"
	"os"
	"testing"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/yola1107/kratos/v2/log"
)

type testWriteSyncer struct {
	output []string
}

func (x *testWriteSyncer) Write(p []byte) (n int, err error) {
	x.output = append(x.output, string(p))
	return len(p), nil
}

func (x *testWriteSyncer) Sync() error {
	return nil
}

func TestLogger(t *testing.T) {
	//syncer := &testWriteSyncer{}
	encoderCfg := zapcore.EncoderConfig{
		//MessageKey:     "msg",
		//LevelKey:       "level",
		//NameKey:        "logger",
		//EncodeLevel:    zapcore.LowercaseLevelEncoder,
		//EncodeTime:     zapcore.ISO8601TimeEncoder,
		//EncodeDuration: zapcore.StringDurationEncoder,

		TimeKey:          "ts",
		LevelKey:         "level",
		NameKey:          "logger",
		CallerKey:        "caller",
		FunctionKey:      zapcore.OmitKey,
		MessageKey:       "msg",
		StacktraceKey:    "stack",
		LineEnding:       zapcore.DefaultLineEnding,
		EncodeLevel:      zapcore.CapitalLevelEncoder,
		EncodeTime:       zapcore.ISO8601TimeEncoder,
		EncodeDuration:   zapcore.StringDurationEncoder,
		EncodeCaller:     zapcore.FullCallerEncoder, // zapcore.FullCallerEncoder,
		ConsoleSeparator: " ",
	}

	options := []zap.Option{
		//zap.AddCaller(),
		//zap.AddCallerSkip(2),
		//zap.AddStacktrace(zap.PanicLevel),
		//zap.Development(),

		zap.AddCaller(),
		zap.AddCallerSkip(2),
		zap.AddStacktrace(zap.PanicLevel),
		zap.WithCaller(true),
	}

	//core := zapcore.NewCore(zapcore.NewJSONEncoder(encoderCfg), syncer, zap.DebugLevel)
	//zlogger := zap.New(core).WithOptions()

	core := zapcore.NewCore(zapcore.NewConsoleEncoder(encoderCfg), zapcore.Lock(os.Stderr), zap.DebugLevel)
	zlogger := zap.New(core, options...)
	logger := NewLogger(zlogger)

	defer func() { _ = logger.Close() }()

	zlogger.WithOptions()

	zlog := log.NewHelper(logger)

	zlog.Debugw("log", "debug")
	zlog.Infow("log", "info")
	zlog.Warnw("log", "warn")
	zlog.Errorw("log", "error")
	zlog.Errorw("log", "error", "except warn")
	zlog.Info("hello world")
	//
	//except := []string{
	//	"{\"level\":\"debug\",\"msg\":\"\",\"log\":\"debug\"}\n",
	//	"{\"level\":\"info\",\"msg\":\"\",\"log\":\"info\"}\n",
	//	"{\"level\":\"warn\",\"msg\":\"\",\"log\":\"warn\"}\n",
	//	"{\"level\":\"error\",\"msg\":\"\",\"log\":\"error\"}\n",
	//	"{\"level\":\"warn\",\"msg\":\"Keyvalues must appear in pairs: [log error except warn]\"}\n",
	//	"{\"level\":\"info\",\"msg\":\"hello world\"}\n", // not {"level":"info","msg":"","msg":"hello world"}
	//}
	//for i, s := range except {
	//	if s != syncer.output[i] {
	//		t.Logf("except=%s, got=%s", s, syncer.output[i])
	//		t.Fail()
	//	}
	//}

	fmt.Printf("--------------\n")

	log.SetLogger(logger)
	log.Debugf("debug")
	log.Infof("info")
	log.Warnf("warn")
	log.Errorf("error")
	log.Fatalf("fatal")

	log.Infof("abc")
}
