package gtable

import (
	"time"

	"github.com/yola1107/kratos/v2/log"
	"github.com/yola1107/kratos/v2/ztest/api-server/internal/conf"
	"github.com/yola1107/kratos/v2/ztest/api-server/internal/room/gplayer"
)

func (t *Table) OnTimer() {
	log.Infof("Stage=%d timeID=%d TimeOut... ", t.stage.curr, t.stage.timerID)

	switch t.stage.curr {
	case conf.StPrepare:
		// t.gameStart()
	case conf.StSendCard:
		// t.notifyAction(false, ACTION)
	case conf.StAction: // 超时操作
		// t.OnAction(t.CurrPlayer(), network.Packet{"action": PLAYER_PACK}, true)
	case conf.StWaitSiderShow: // 比牌操作超时
		// t.OnAction(t.CurrPlayer(), network.Packet{"action": PLAYER_OK_SIDER_SHOW, "allow": false}, true)
	case conf.StSiderShow: // 操作之后等待时间
		// t.notifyAction(true, ACTION)
	case conf.StWaitEnd:
		// t.gameEnd()
	case conf.StEnd: // 游戏结束后判断
		// t.clearAnomalyPlayers()
		// t.Reset()
		// t.checkReady()
		// t.mLog.End(fmt.Sprintf("结束清理完成。"))
	}
}

func (t *Table) updateStage(state int32) {
	timer := t.repo.GetTimer()
	timer.Cancel(t.stage.timerID) // 取消当前阶段的定时任务
	t.stage.last = t.stage.curr
	t.stage.curr = state
	t.stage.startTime = time.Now()
	t.stage.duration = conf.GetStageTimeout(state)
	t.stage.timerID = timer.Once(t.stage.duration, t.OnTimer)
	log.Infof("stage changed. timerID(%d) stage:(%d -> %d) ", t.stage.timerID, t.stage.last, t.stage.curr)
}

func (t *Table) canStart() bool {
	if t.sitCnt >= 2 {
		return true
	}
	return false
}

func (t *Table) start() {
}

func (t *Table) canEnter(p *gplayer.Player) bool {
	return false
}

func (t *Table) canExit(p *gplayer.Player) bool {
	return false
}
