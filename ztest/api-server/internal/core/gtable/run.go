package gtable

import (
	"github.com/yola1107/kratos/v2/log"
	"github.com/yola1107/kratos/v2/ztest/api-server/internal/conf"
)

//// OnTimer 桌子定时
//func (t *Table) OnTimer() {
//	// 超时处理
//	if ext.GetTick() > t.nTimeOut {
//		switch t.nStage {
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

func (t *Table) OnTimer() {
	oldState := t.stage
	defer func() {
		log.Infof("TimeOut... %v->%v", oldState, t.stage)
	}()

	switch t.stage {
	case conf.StPrepare:
		//t.gameStart()
	case conf.StSendCard:
		//t.notifyAction(false, ACTION)
	case conf.StAction: // 超时操作
		//t.OnAction(t.CurrPlayer(), network.Packet{"action": PLAYER_PACK}, true)
	case conf.StWaitSiderShow: // 比牌操作超时
		//t.OnAction(t.CurrPlayer(), network.Packet{"action": PLAYER_OK_SIDER_SHOW, "allow": false}, true)
	case conf.StSiderShow: // 操作之后等待时间
		//t.notifyAction(true, ACTION)
	case conf.StWaitEnd:
		//t.gameEnd()
	case conf.StEnd: // 游戏结束后判断
		//t.clearAnomalyPlayers()
		//t.Reset()
		//t.checkReady()
		//t.mLog.End(fmt.Sprintf("结束清理完成。"))
	}
}
