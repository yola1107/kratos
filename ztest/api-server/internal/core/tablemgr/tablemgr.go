package tablemgr

import (
	"github.com/yola1107/kratos/v2/ztest/api-server/internal/core/tablemgr/gtable"
)

// TableMgr 管理多个桌子的日志
type TableMgr struct {
	Tables map[int64]*gtable.Table
}
