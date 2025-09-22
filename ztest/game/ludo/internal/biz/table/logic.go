package table

import (
	"fmt"
	"time"

	"github.com/yola1107/kratos/v2/library/xgo"
	"github.com/yola1107/kratos/v2/log"
	v1 "github.com/yola1107/kratos/v2/ztest/game/ludo/api/helloworld/v1"
	"github.com/yola1107/kratos/v2/ztest/game/ludo/internal/biz/player"
	"github.com/yola1107/kratos/v2/ztest/game/ludo/internal/conf"
	"github.com/yola1107/kratos/v2/ztest/game/ludo/internal/model"
)

/*
	游戏主逻辑
*/

// MinStartPlayerCnt 最小开局人数
const MinStartPlayerCnt = 2

func (t *Table) OnTimer() {
	state := t.stage.GetState()
	log.Debugf("[Stage] OnTimer timeout. St:%v active=%d Tb=%v", state, t.active, t.Desc())
	switch state {
	case StWait:
		t.checkCanStart()
	case StReady:
		t.onGameStart()
	case StSendCard:
		t.onSendCardTimeout()
	case StDice:
		t.onDiceTimeout()
	case StMove:
		t.onMoveTimeout()
	case StResult:
		t.gameEnd()
	default:
		log.Errorf("unhandled stage timeout: %v", state)
	}
}

func (t *Table) updateStage(state StageID) {
	timeout := time.Duration(state.Timeout()) * time.Second
	t.updateStageWith(state, timeout)
}

func (t *Table) updateStageWith(state StageID, duration time.Duration) {
	// 取消之前定时器，启动新定时器
	t.repo.GetTimer().Cancel(t.stage.GetTimerID())
	timerID := t.repo.GetTimer().Once(duration, t.OnTimer)

	t.stage.Set(state, duration, timerID)
	t.mLog.stage(t.stage.Desc(), t.active)
	// log.Debugf("[Stage] ====> %v, active=%d ", t.stage.Desc(), t.active)
}

// 判断是否满足开局条件，满足则进入准备阶段
func (t *Table) checkCanStart() {
	if t.stage.GetState() != StWait || !t.hasEnoughReadyPlayers() {
		return
	}

	log.Infof("Table %s: Enough players ready (%d/%d), transitioning to StReady", t.Desc(), t.sitCnt, t.MaxCnt)

	// 幂等保护由 updateStage 保证
	t.updateStage(StReady)
}

func (t *Table) hasEnoughReadyPlayers() bool {
	count := 0
	baseMoney := t.repo.GetRoomConfig().Game.BaseMoney
	for _, p := range t.seats {
		if p != nil && p.IsReady() && p.GetAllMoney() >= baseMoney {
			count++
		}
	}
	return count >= MinStartPlayerCnt && count%2 == 0
}

// 开局流程
func (t *Table) onGameStart() {
	if t.stage.GetState() != StReady {
		t.updateStage(StWait)
		return
	}

	canStart, seats := t.getReadySeats()
	if !canStart {
		t.updateStage(StWait)
		return
	}

	t.intoGaming(seats)
	t.calcBanker(seats)
	t.dispatchCard(seats)
	t.updateStage(StSendCard)

	infos := make([]string, 0, len(seats))
	for _, p := range seats {
		infos = append(infos, fmt.Sprintf("%d:%d", p.GetPlayerID(), p.GetChairID()))
	}

	log.Debugf("******** <游戏开始> %s GamerInfo=%v", t.Desc(), infos)
	t.mLog.begin(t.Desc(), t.repo.GetRoomConfig().Game.BaseMoney, t.seats, infos)
}

// 获取所有满足准备和资金条件的玩家
func (t *Table) getReadySeats() (bool, []*player.Player) {
	baseMoney := t.repo.GetRoomConfig().Game.BaseMoney
	readySeats := make([]*player.Player, 0, len(t.seats))
	for _, p := range t.seats {
		if p != nil && p.IsReady() && p.GetAllMoney() >= baseMoney {
			readySeats = append(readySeats, p)
		}
	}
	count := len(readySeats)
	return count >= MinStartPlayerCnt && count%2 == 0, readySeats
}

// 检查是否自动准备
func (t *Table) checkAutoReady(p *player.Player) {
	if !t.repo.GetRoomConfig().Game.AutoReady {
		return
	}
	if p != nil && !p.IsReady() && p.GetAllMoney() >= t.repo.GetRoomConfig().Game.BaseMoney {
		p.SetReady()
	}
}

// 自动准备逻辑
func (t *Table) checkAutoReadyAll() {
	if !t.repo.GetRoomConfig().Game.AutoReady {
		return
	}
	baseMoney := t.repo.GetRoomConfig().Game.BaseMoney
	for _, p := range t.seats {
		if p != nil && !p.IsReady() && p.GetAllMoney() >= baseMoney {
			p.SetReady()
		}
	}
}

// 扣钱并设置玩家游戏状态
func (t *Table) intoGaming(seats []*player.Player) {
	baseMoney := t.repo.GetRoomConfig().Game.BaseMoney
	var seatColors []int32
	for _, p := range seats {
		if p == nil {
			continue
		}
		p.SetGaming()
		if !p.IntoGaming(baseMoney) {
			log.Errorf("intoGaming error. p:%+v baseMoney=%.1f", p.Desc(), baseMoney)
		}
		color := p.GetChairID()
		p.SetColor(color)
		seatColors = append(seatColors, color)
	}

	// 初始化棋盘
	t.board = model.NewBoard(seatColors, 4, conf.IsFastMode())

	// 快速场定时器
	if conf.IsFastMode() {
		t.timerJobFast = time.AfterFunc(time.Minute*5, func() { t.settle(1) })
	}

	for _, p := range seats {
		if p == nil || !p.IsGaming() {
			continue
		}
		color := p.GetColor()
		p.SetPieces(t.board.GetPieceIDsByColor(color))
		t.colorMap[color] = p.GetPlayerID()
	}

}

// 计算庄家
func (t *Table) calcBanker(seats []*player.Player) {
	idx := xgo.RandInt(0, len(seats))
	t.active = seats[idx].GetChairID()
	t.first = t.active
}

// 发牌流程
func (t *Table) dispatchCard(seats []*player.Player) {
	t.dispatchCardPush(seats)
}

func (t *Table) onSendCardTimeout() {
	t.updateStage(StDice)
	t.broadcastActivePlayerPush()
}

func (t *Table) onDiceTimeout() {
	p := t.GetActivePlayer()
	if p == nil {
		log.Errorf("onDiceTimeout: no active player at table %v", t.Desc())
		return
	}
	if !p.IsGaming() {
		log.Errorf("onDiceTimeout: no active gaming player at table %v", t.Desc())
		return
	}
	t.OnDiceReq(p, &v1.DiceReq{Uid: p.GetPlayerID()}, true)
}

func (t *Table) onMoveTimeout() {
	p := t.GetActivePlayer()
	if p == nil {
		log.Errorf("onMoveTimeout: no active player at table %v", t.Desc())
		return
	}
	if !p.IsGaming() {
		log.Errorf("onMoveTimeout: no active gaming player at table %v", t.Desc())
		return
	}

	// 提前准备所需数据
	uid := p.GetPlayerID()
	dice := p.UnusedDice()
	color := p.GetColor()
	chair := p.GetChairID()

	t.repo.GetLoop().Post(func() {
		if t.active != chair || t.board == nil || t.stage.GetState() != StMove {
			return
		}
		id, x := model.FindBestMoveSequence(t.board, dice, color)
		if id <= -1 || x <= -1 {
			log.Errorf("onMoveTimeout: 找不到可移动的路径. tb=%v, p=%v", t.Desc(), p.Desc())
			return
		}
		t.OnMoveReq(p, &v1.MoveReq{UserId: uid, PieceId: id, DiceValue: x}, true)
	})
}

func (t *Table) gameEnd() {
	// 清理数据
	t.Reset()

	// 状态进入 StEnd
	t.updateStage(StWait)

	// // todo delete test
	// t.SendPacketToAll(v1.GameCommand_Nothing, nil) // todo test ai exit

	// 检查踢人
	t.checkKick()

	// 是否自动准备
	t.checkAutoReadyAll()

	// // 是否可以下一局
	// t.checkCanStart()

	// log
	log.Debugf("结束清理完成。tb=%v \n\n\n", t.Desc())
	t.mLog.end(fmt.Sprintf("结束清理完成。%s %s", t.Desc(), logPlayers(t.seats)))
}

func (t *Table) settle(endTy int32) *SettleObj {
	t.updateStage(StResult)
	t.broadcastResult(nil)
	return nil
}
