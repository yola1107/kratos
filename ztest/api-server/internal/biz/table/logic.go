package table

import (
	"fmt"
	"time"

	"github.com/yola1107/kratos/v2/log"
	v1 "github.com/yola1107/kratos/v2/ztest/api-server/api/helloworld/v1"
	"github.com/yola1107/kratos/v2/ztest/api-server/internal/biz/player"
)

/*
	游戏主逻辑
*/

// MinStartPlayerCnt 最小开局人数
const MinStartPlayerCnt = 2

type Stage struct {
	State    StageID
	Prev     StageID
	TimerID  int64
	StartAt  time.Time
	Duration time.Duration
}

func (s *Stage) Remaining() time.Duration {
	return max(s.Duration-time.Since(s.StartAt), time.Millisecond)
}

func (s *Stage) Set(state StageID, duration time.Duration, timerID int64) {
	s.Prev = s.State
	s.State = state
	s.StartAt = time.Now()
	s.Duration = duration
	s.TimerID = timerID
}

func (t *Table) OnTimer() {
	log.Debugf("TimeOut Stage ... %v ", t.stage.State)

	switch t.stage.State {
	case StWait:
		// StWait 状态通常不会超时的，除非没有玩家开局，可选踢掉长时间占桌不开局的玩家
		log.Infof("StWait timmeout.")
	case StReady:
		t.onGameStart()
	case StSendCard:
		t.onSendCardTimeout()
	case StAction:
		t.onActionTimeout()
	case StSideShow:
		t.onSideShowTimeout()
	case StSideShowAni:
		t.onSideShowAniTimeout()
	case StWaitEnd:
		t.gameEnd()
	case StEnd:
		t.onEndTimeout()
	default:
		log.Errorf("unhandled stage timeout: %v", t.stage.State)
	}
}

func (t *Table) updateStage(state StageID) {
	timeout := time.Duration(state.Timeout()) * time.Second
	t.updateStageWith(state, timeout)
}

func (t *Table) updateStageWith(state StageID, duration time.Duration) {
	t.repo.GetTimer().Cancel(t.stage.TimerID)              // 取消当前定时器
	timerID := t.repo.GetTimer().Once(duration, t.OnTimer) // 启动新定时器
	t.stage.Set(state, duration, timerID)                  // 设置阶段

	// 日志
	t.mLog.stage(t.stage.Prev, t.stage.State, t.active)
	log.Debugf("*** %s -> %s  dur=%v", t.stage.Prev.String(), t.stage.State.String(), duration)
}

func (t *Table) checkStartGame() {
	if t.stage.State != StWait {
		return
	}

	canStart, _, chairs := t.calcReadyPlayer()
	if !canStart {
		return
	}

	// 准备开局
	t.updateStage(StReady)
	log.Debugf("=> 准备开局. ReadyCnt=%d chairs:%s", len(chairs), chairs)
}

func (t *Table) onGameStart() {
	// 再次检查是否可进行游戏; 兜底回退到StWait
	can, canGameSeats, infos := t.calcReadyPlayer()
	if !can || t.stage.State != StReady {
		t.updateStage(StWait)
		return
	}

	// 扣钱
	t.intoGaming(canGameSeats)

	// 计算庄家及操作玩家
	t.calcBanker()

	// 发牌
	t.dispatchCard(canGameSeats)

	// 发牌状态倒计时3s
	t.updateStage(StSendCard)

	log.Debugf("******** <游戏开始> %s canGameSeats:%+v", t.Desc(), infos)
	t.mLog.begin(t.Desc(), t.curBet, canGameSeats, infos)
}

// 检查用户是否可以开局
func (t *Table) calcReadyPlayer() (bool, []*player.Player, []string) {
	canGameInfo := []string(nil)
	canGameSeats := []*player.Player(nil)
	for _, v := range t.seats {
		if v == nil || v.GetAllMoney() < t.curBet || !v.IsReady() {
			continue
		}
		canGameSeats = append(canGameSeats, v)
		canGameInfo = append(canGameInfo, fmt.Sprintf("%d:%d", v.GetPlayerID(), v.GetChairID()))
	}

	return len(canGameSeats) >= MinStartPlayerCnt, canGameSeats, canGameInfo
}

// 检查是否自动准备
func (t *Table) checkAutoReady(p *player.Player) {
	if !t.repo.GetRoomConfig().Game.AutoReady {
		return
	}
	if p != nil && !p.IsReady() && p.GetAllMoney() >= t.curBet {
		p.SetReady()
	}
}

func (t *Table) checkAutoReadyAll() {
	if !t.repo.GetRoomConfig().Game.AutoReady {
		return
	}
	for _, p := range t.seats {
		if p != nil && !p.IsReady() && p.GetAllMoney() >= t.curBet {
			p.SetReady()
		}
	}
}

// 扣钱 （或处理可以进行游戏的玩家状态等逻辑）
func (t *Table) intoGaming(canGameSeats []*player.Player) {
	for _, p := range canGameSeats {
		p.SetGaming() //
		if !p.IntoGaming(t.curBet) {
			log.Errorf("intoGaming error. p:%+v currBet=%.1f", p.Desc(), t.curBet)
			// continue
		} else {
			t.totalBet += t.curBet
		}
	}
}

// 计算庄家位置
func (t *Table) calcBanker() {
	next := t.NextPlayer(t.banker)
	if next == nil {
		log.Errorf("calcBanker err. banker=%v", t.banker)
		return
	}
	t.curRound = 1
	t.banker = next.GetChairID()
	t.active = t.banker
	t.first = t.active
	t.broadcastSetBankerRsp()
}

// 发牌
func (t *Table) dispatchCard(canGameSeats []*player.Player) {
	// 洗牌
	t.cards.Shuffle()

	// 发牌
	for _, p := range canGameSeats {
		p.AddCards(t.cards.DispatchCards(3))
	}

	// 发牌广播
	t.dispatchCardPush(canGameSeats)
}

func (t *Table) onSendCardTimeout() {
	t.updateStage(StAction)
	t.broadcastActivePlayerPush()
}

func (t *Table) onActionTimeout() {
	t.OnActionReq(t.GetActivePlayer(), &v1.ActionReq{Action: v1.ACTION_PACK}, true)
}

func (t *Table) onSideShowTimeout() {
	t.OnActionReq(t.GetActivePlayer(), &v1.ActionReq{Action: v1.ACTION_SIDE_REPLY, SideReplyAllow: false}, true)
}

// 比牌赢家操作
func (t *Table) onSideShowAniTimeout() {
	t.updateStage(StAction)
	t.broadcastActivePlayerPush()
	t.checkRound(t.active)
}

/* 游戏结束 */
func (t *Table) gameEnd() {
	// 胜利的玩家
	var winner *player.Player
	for _, seat := range t.seats {
		if seat != nil && seat.IsGaming() {
			winner = seat
			break
		}
	}

	if winner == nil {
		t.updateStage(StEnd)
		log.Errorf("gameEnd err. winner=%+v", winner)
		return
	}

	// 结算
	profit := winner.Settle(t.totalBet)
	// t.Broadcast(-1, packet)
	// t.SendShowCard()
	t.broadcastResult()
	t.mLog.settle(profit)
	t.updateStage(StEnd)
}

func (t *Table) onEndTimeout() {
	// 游戏结束后判断
	t.checkKick()
	t.Reset()

	// 状态进入 StWait
	t.updateStage(StWait)

	// 是否自动准备
	t.checkAutoReadyAll()

	// 是否可以下一局
	t.checkStartGame()

	log.Debugf("结束清理完成。\n")
	t.mLog.end(fmt.Sprintf("结束清理完成。%s", t.Desc()))
}
