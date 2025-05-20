package gtable

import (
	"github.com/yola1107/kratos/v2/log"
	"github.com/yola1107/kratos/v2/ztest/api-server/internal/conf"
)

//// OnTimer 桌子定时
//func (tb *Table) OnTimer() {
//	// 超时处理
//	if ext.GetTick() > tb.nTimeOut {
//		switch tb.nStage {
//		case conf.StPrepare:
//			//tl.prepareStart()
//		case conf.StSendCard:
//			//tl.onSendCardTimeout()
//		case conf.StGetCard:
//			//tl.onGetCardTimeout()
//		case conf.StPlayCard:
//			//tl.onPlayCardTimeout()
//		case conf.StDismiss:
//			//tl.onDismissProTimeout()
//		case conf.StSmallEnd:
//			//tl.onSmallEndTimeout()
//		case conf.StResult:
//			//tl.onResultTimeout()
//		default:
//		}
//	}
//}

func (tb *Table) OnTimer() {
	oldState := tb.stage
	defer func() {
		log.Infof("TimeOut... %v->%v", oldState, tb.stage)
	}()

	switch tb.stage {
	case conf.StPrepare:
		//tb.gameStart()
	case conf.StSendCard:
		//tb.notifyAction(false, ACTION)
	case conf.StAction: // 超时操作
		//tb.OnAction(tb.CurrPlayer(), network.Packet{"action": PLAYER_PACK}, true)
	case conf.StWaitSiderShow: // 比牌操作超时
		//tb.OnAction(tb.CurrPlayer(), network.Packet{"action": PLAYER_OK_SIDER_SHOW, "allow": false}, true)
	case conf.StSiderShow: // 操作之后等待时间
		//tb.notifyAction(true, ACTION)
	case conf.StWaitEnd:
		//tb.gameEnd()
	case conf.StEnd: // 游戏结束后判断
		//tb.clearAnomalyPlayers()
		//tb.Reset()
		//tb.checkReady()
		//tb.mLog.End(fmt.Sprintf("结束清理完成。"))
	}
}
