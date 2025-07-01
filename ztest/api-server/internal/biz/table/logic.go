package table

import (
	"fmt"
	"sync"
	"time"

	"github.com/yola1107/kratos/v2/library/ext"
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
	mu       sync.RWMutex
	State    StageID
	Prev     StageID
	TimerID  int64
	StartAt  time.Time
	Duration time.Duration
}

func (s *Stage) Remaining() time.Duration {
	s.mu.RLock()
	defer s.mu.RUnlock()
	elapsed := time.Since(s.StartAt)
	if elapsed > s.Duration {
		return 0
	}
	return s.Duration - elapsed
}

func (s *Stage) GetState() StageID {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.State
}

func (s *Stage) GetTimerID() int64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.TimerID
}

func (s *Stage) Snap() (StageID, StageID, time.Duration, time.Time, int64) {
	s.mu.RLock()
	prev, state, dur, at, timerID := s.Prev, s.State, s.Duration, s.StartAt, s.TimerID
	s.mu.RUnlock()
	return prev, state, dur, at, timerID
}

func (s *Stage) Desc() string {
	prev, state, duration, _, _ := s.Snap()
	return fmt.Sprintf("[%v->%+v, %+v -> %v, dur=%v]",
		int32(prev), int32(state), prev, state, duration)
}

func (s *Stage) Set(state StageID, duration time.Duration, timerID int64) {
	s.mu.Lock() // 写锁
	defer s.mu.Unlock()
	s.Prev = s.State
	s.State = state
	s.StartAt = time.Now()
	s.Duration = duration
	s.TimerID = timerID
}

func (t *Table) OnTimer() {
	state := t.stage.GetState()
	// timerID := t.stage.GetTimerID()
	// log.Debugf("[Stage] OnTimer timeout. St:%v TimerID=%d", state, timerID)

	switch state {
	case StWait:
		// log.Debugf("StWait timeout. tb:%v ", t.Desc())
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
		log.Errorf("unhandled stage timeout: %v ", state)
	}
}

func (t *Table) updateStage(state StageID) {
	timeout := time.Duration(state.Timeout()) * time.Second
	t.updateStageWith(state, timeout)
}

func (t *Table) updateStageWith(state StageID, duration time.Duration) {
	// 获取当前定时器ID并取消
	currentTimerID := t.stage.GetTimerID()
	t.repo.GetTimer().Cancel(currentTimerID)

	// 启动新定时器
	timerID := t.repo.GetTimer().Once(duration, t.OnTimer)

	// 更新阶段
	t.stage.Set(state, duration, timerID)

	// 日志
	t.mLog.stage(t.stage.Desc(), t.active)
	// log.Debugf("[Stage] ====> %v, timerID: %d->%d", t.stage.Desc(), currentTimerID, timerID)
}

func (t *Table) checkCanStart() {
	if t.stage.GetState() != StWait {
		return
	}

	if canStart := t.checkReadyPlayer(); !canStart {
		return
	}

	// 准备开局
	t.updateStage(StReady)
}

func (t *Table) onGameStart() {
	// 再次检查是否可进行游戏; 兜底回退到StWait
	can, seats, infos := t.checkReadyInfos()
	if !can || t.stage.GetState() != StReady {
		t.updateStage(StWait)
		return
	}

	// 扣钱
	t.intoGaming(seats)

	// 计算庄家及操作玩家
	t.calcBanker(seats)

	// 发牌
	t.dispatchCard(seats)

	// 发牌状态倒计时3s
	t.updateStage(StSendCard)

	log.Debugf("******** <游戏开始> %s GamerInfo=%+v all=%v", t.Desc(), infos, logPlayers(t.seats))
	t.mLog.begin(t.Desc(), t.curBet, t.seats, infos)
}

// 检查用户是否可以开局
func (t *Table) checkReadyPlayer() bool {
	okCnt := 0
	for _, v := range t.seats {
		if v == nil || !v.IsReady() || v.GetAllMoney() < t.curBet {
			continue
		}
		okCnt++
	}
	return okCnt >= MinStartPlayerCnt
}

func (t *Table) checkReadyInfos() (bool, []*player.Player, []string) {
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
func (t *Table) intoGaming(seats []*player.Player) {
	for _, p := range seats {
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
func (t *Table) calcBanker(seats []*player.Player) {
	idx := ext.RandInt(0, len(seats))
	t.banker = seats[int32(idx)].GetChairID()
	t.curRound = 1
	t.active = t.banker
	t.first = t.active
	t.broadcastSetBankerRsp()
}

// 发牌
func (t *Table) dispatchCard(seats []*player.Player) {
	// 洗牌
	t.cards.Shuffle()

	// 发牌
	for _, p := range seats {
		p.AddCards(t.cards.DispatchCards(3))
	}

	// 发牌广播
	t.dispatchCardPush(seats)
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
		log.Errorf("gameEnd err. tb=%+v", t.Desc())
		return
	}

	// 结算
	winner.Settle(t.totalBet)
	// t.Broadcast(-1, packet)
	// t.SendShowCard()
	t.broadcastResult()
	// log.Debugf("gameEnd tb=%s winner=%+v", t.Desc(), winner.Desc())
	t.mLog.settle(winner)
	t.updateStage(StEnd)
}

func (t *Table) onEndTimeout() {
	// 游戏结束后踢人
	t.checkKick()

	// 重置数据
	t.Reset()

	log.Debugf("结束清理完成。tb=%v \n", t.Desc())
	t.mLog.end(fmt.Sprintf("结束清理完成。%s %s", t.Desc(), logPlayers(t.seats)))

	// 状态进入 StWait
	t.updateStage(StWait)

	// 是否自动准备
	t.checkAutoReadyAll()

	// 是否可以下一局
	t.checkCanStart()
}
