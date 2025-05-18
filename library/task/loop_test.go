package task

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/yola1107/kratos/v2/log"
)

func init() {
	log.SetLogger(log.NewStdLogger(os.Stdout))
}

func TestLoop(t *testing.T) {
	l := NewLoop(2)
	l.Start()

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
		val, err := l.PostAndWait(func() ([]byte, error) {
			defer func() {
				t.Log("panic recovered")
				close(done)
			}()
			panic("oops2")
			return []byte("hello"), nil
		})
		t.Logf("panic recovered val=%+v ,err=%+v", val, err)
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Fatal("panic job did not finish")
		}
	})

	t.Run("PostAndWaitAny panic inside job is recovered", func(t *testing.T) {
		done := make(chan struct{})
		x := l.PostAndWaitAny(func() any {
			defer func() {
				t.Log("panic recovered")
				close(done)
			}()
			panic("oops3")
		})
		t.Logf("panic recovered x=%+v", x)
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Fatal("panic job did not finish")
		}
	})
}
