package gtable

import (
	"github.com/yola1107/kratos/v2/library/ext"
	"github.com/yola1107/kratos/v2/ztest/api-server/internal/conf"
)

// OnTimer 桌子定时
func (tb *Table) OnTimer() {
	// 超时处理
	if ext.GetTick() > tb.nTimeOut {
		switch tb.nStage {
		case conf.StPrepare:
			//tl.prepareStart()
		case conf.StSendCard:
			//tl.onSendCardTimeout()
		case conf.StGetCard:
			//tl.onGetCardTimeout()
		case conf.StPlayCard:
			//tl.onPlayCardTimeout()
		case conf.StDismiss:
			//tl.onDismissProTimeout()
		case conf.StSmallEnd:
			//tl.onSmallEndTimeout()
		case conf.StResult:
			//tl.onResultTimeout()
		default:
		}
	}
}
