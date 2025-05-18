package work

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/yola1107/kratos/v2/log"
)

func init() {
	log.SetLogger(log.NewStdLogger(os.Stdout))
}

func TestAntsLoop(t *testing.T) {
	l := NewAntsLoop(2)
	err := l.Start()
	require.NoError(t, err)
	defer l.Stop()

	t.Run("start more times", func(t *testing.T) {
		err = l.Start()
		require.NoError(t, err)
	})

	t.Run("Post simple task", func(t *testing.T) {
		done := make(chan struct{})
		l.Post(func() {
			close(done)
		})
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Fatal("task not finished")
		}
	})

	t.Run("PostAndWait returns expected value", func(t *testing.T) {
		val, err := l.PostAndWait(func() ([]byte, error) {
			return []byte("hello"), nil
		})
		require.NoError(t, err)
		require.Equal(t, []byte("hello"), val)
	})

	t.Run("Post panic inside job is recovered", func(t *testing.T) {
		done := make(chan struct{})
		l.Post(func() {
			defer func() {
				t.Log("panic recovered")
				close(done)
			}()
			panic("oops")
		})
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Fatal("panic job did not finish")
		}
	})

	t.Run("PostAndWait panic inside job is recovered", func(t *testing.T) {
		done := make(chan struct{})
		d, err := l.PostAndWait(func() ([]byte, error) {
			defer func() {
				t.Log("panic recovered")
				close(done)
			}()
			panic("oops2")
			return []byte("hello"), nil
		})
		t.Logf("panic recovered d=%+v ,err=%+v", d, err)
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Fatal("panic job did not finish")
		}
	})

	t.Run("fallback when stopped", func(t *testing.T) {
		l.Stop()
		// Stop 异步释放，等一会
		time.Sleep(100 * time.Millisecond)

		executed := make(chan struct{})
		l.PostCtx(context.Background(), func() {
			close(executed)
		})

		select {
		case <-executed:
		case <-time.After(time.Second):
			t.Fatal("fallback job did not run")
		}
		// 重新启动方便后面测试
		err := l.Start()
		require.NoError(t, err)
	})

	t.Run("context cancel works in PostAndWaitCtx", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()

		_, err := l.PostAndWaitCtx(ctx, func() ([]byte, error) {
			time.Sleep(200 * time.Millisecond)
			return []byte("slow"), nil
		})
		require.Error(t, err)
		require.True(t, errors.Is(err, context.DeadlineExceeded))
	})
}
