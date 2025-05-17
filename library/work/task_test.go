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
	loop := NewAntsLoop(2)
	err := loop.Start()
	require.NoError(t, err)

	t.Run("Post simple task", func(t *testing.T) {
		done := make(chan struct{})
		loop.Post(func() {
			close(done)
		})
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Fatal("task not finished")
		}
	})

	t.Run("PostAndWait returns expected value", func(t *testing.T) {
		val, err := loop.PostAndWait(func() ([]byte, error) {
			return []byte("hello"), nil
		})
		require.NoError(t, err)
		require.Equal(t, []byte("hello"), val)
	})

	t.Run("panic inside job is recovered", func(t *testing.T) {
		loop.Post(func() {
			defer t.Log("-------------------")
			panic("oops")
		})
		// 只要不 panic 即可通过
	})

	t.Run("fallback when stopped", func(t *testing.T) {
		loop := NewAntsLoop(1)
		loop.Stop()
		time.Sleep(50 * time.Millisecond)

		executed := make(chan struct{})
		loop.PostCtx(context.Background(), func() {
			close(executed)
		})

		select {
		case <-executed:
		case <-time.After(time.Second):
			t.Fatal("fallback job did not run")
		}
	})

	t.Run("context cancel works in PostAndWaitCtx", func(t *testing.T) {
		loop := NewAntsLoop(1)
		_ = loop.Start()
		defer loop.Stop()

		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()

		_, err := loop.PostAndWaitCtx(ctx, func() ([]byte, error) {
			time.Sleep(200 * time.Millisecond)
			return []byte("slow"), nil
		})
		require.Error(t, err)
		require.True(t, errors.Is(err, context.DeadlineExceeded))
	})
}
