package gtimer

import (
	"context"

	"github.com/yola1107/kratos/v2/library/work"
)

/*
	全局定时器+任务池
*/

var (
	ws work.IWorkStore
)

func GetWorkStore() work.IWorkStore {
	return ws
}

func Init() {
	ws = work.NewWorkStore(context.Background(), 10000)
	if err := ws.Start(); err != nil {
		panic(err)
	}
}

func Close() {
	ws.Stop()
}
