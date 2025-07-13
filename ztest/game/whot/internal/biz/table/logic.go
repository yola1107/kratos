package table

import (
	"fmt"
	"math"
	"time"

	"github.com/yola1107/kratos/v2/library/ext"
	"github.com/yola1107/kratos/v2/log"
	v1 "github.com/yola1107/kratos/v2/ztest/game/whot/api/helloworld/v1"
	"github.com/yola1107/kratos/v2/ztest/game/whot/internal/biz/player"
)

/*
	游戏主逻辑
*/

// MinStartPlayerCnt 最小开局人数
const MinStartPlayerCnt = 2

func (t *Table) OnTimer() {
	state := t.stage.GetState()
	// log.Debugf("[Stage] OnTimer timeout. St:%v TimerID=%d", state, t.stage.GetTimerID())
	switch state {
	case StWait:
		// 等待阶段无动作或日志
	case StReady:
		t.onGameStart()
	case StSendCard:
		t.onSendCardTimeout()
	case StPlaying:
		t.onActionTimeout()
	case StWaitEnd:
		t.gameEnd()
	case StEnd:
		t.onEndTimeout()
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
	// log.Debugf("[Stage] ====> %v, timerID: %d", t.stage.Desc(), timerID)
}

// 判断是否满足开局条件，满足则进入准备阶段
func (t *Table) checkCanStart() {
	if t.stage.GetState() != StWait || !t.hasEnoughReadyPlayers() {
		return
	}
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
	return count >= MinStartPlayerCnt
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
	return len(readySeats) >= MinStartPlayerCnt, readySeats
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
	for _, p := range seats {
		if p == nil {
			continue
		}
		p.SetGaming()
		if !p.IntoGaming(baseMoney) {
			log.Errorf("intoGaming error. p:%+v baseMoney=%.1f", p.Desc(), baseMoney)
		}

		t.sendMatchOk(p)
	}
}

// 计算庄家
func (t *Table) calcBanker(seats []*player.Player) {
	idx := ext.RandInt(0, len(seats))
	t.active = seats[idx].GetChairID()
	t.first = t.active
}

// 发牌流程
func (t *Table) dispatchCard(seats []*player.Player) {
	t.cards.Shuffle()

	for _, p := range seats {
		p.AddCards(t.cards.DispatchCards(5))
	}

	bottom := t.cards.SetBottom()
	t.currCard = bottom[0]
	leftNum := t.cards.GetCardNum()

	t.dispatchCardPush(seats, bottom, leftNum)
}

func (t *Table) onSendCardTimeout() {
	t.updateStage(StPlaying)
	t.broadcastActivePlayerPush()
}

func (t *Table) onActionTimeout() {
	p := t.GetActivePlayer()
	if p == nil || !p.IsGaming() {
		log.Errorf("onActionTimeout: no active gaming player at table %v", t.Desc())
		return
	}

	req, err := t.makeAutoActionReq(p)
	if err != nil {
		log.Errorf("onActionTimeout: failed to generate action req: %v", err)
		return
	}

	t.OnPlayerActionReq(p, req, true)
}

func (t *Table) makeAutoActionReq(p *player.Player) (*v1.PlayerActionReq, error) {
	ops := t.getCanOp(p)
	if len(ops) == 0 {
		return nil, fmt.Errorf("no available options: player=%v table=%v", p.Desc(), t.Desc())
	}

	op := ops[ext.RandInt(0, len(ops))]

	req := &v1.PlayerActionReq{
		UserId: p.GetPlayerID(),
		Action: op.Action,
	}

	switch op.Action {
	case v1.ACTION_PLAY_CARD:
		if len(op.Cards) > 0 {
			req.OutCard = op.Cards[ext.RandInt(0, len(op.Cards))]
		}
	case v1.ACTION_DRAW_CARD:
		// no extra fields
	case v1.ACTION_DECLARE_SUIT:
		if len(op.Suits) > 0 {
			req.DeclareSuit = op.Suits[ext.RandInt(0, len(op.Suits))]
		}
	case v1.ACTION_SKIP_TURN:
		// no extra fields
	default:
		return nil, fmt.Errorf("unexpected action=%v for player=%v", op.Action, p.Desc())
	}

	return req, nil
}

func (t *Table) gameEnd() {
	obj := t.settle() //

	t.broadcastResult(obj) //

	t.setSitStatus() // 重置状态

	// 保存每一局记录 // todo
	t.checkKick() // 检查踢人

	t.Reset() // 清理数据

	log.Debugf("结束清理完成。tb=%v", t.Desc())
	t.mLog.end(fmt.Sprintf("结束清理完成。%s %s", t.Desc(), logPlayers(t.seats)))

	t.updateStage(StEnd)
}

func (t *Table) settle() *SettleObj {
	winner, endType := t.calcWinner()
	if winner == nil {
		log.Errorf("settle failed: no valid winner. tb=%+v", t.Desc())
		return nil
	}

	conf := t.repo.GetRoomConfig().Game
	settle := &SettleObj{
		Winner:    winner,
		Users:     t.GetGamers(),
		BaseScore: conf.BaseMoney,
		TaxRate:   conf.Fee,
		EndType:   endType,
	}

	if err := settle.Settle(); err != nil {
		log.Errorf("settle error: %v", err)
		return nil
	}

	// 可选保存结果供查询：
	// t.settleObj = settle

	tax := settle.TaxFee
	win := settle.WinScore
	winner.AddMoney(win)

	t.mLog.settle(winner, win, tax, ext.ToJSON(settle.GetResult()))
	log.Debugf("settle. tb=%s winner=%+v win=%v fee=%.1f info=%v",
		t.Desc(), winner.Desc(), win, tax, settle.GetResult())
	return settle
}

func (t *Table) calcWinner() (*player.Player, v1.FINISH_TYPE) {
	var (
		winner   *player.Player
		minScore int32 = math.MaxInt32
	)

	for _, p := range t.GetGamers() {
		if score := p.GetHandScore(); winner == nil || score < minScore {
			winner = p
			minScore = score
		}
	}

	endType := v1.FINISH_TYPE_DECK_EMPTY
	if winner != nil && minScore == 0 {
		endType = v1.FINISH_TYPE_PLAYER_HAND_EMPTY
	}
	return winner, endType
}

func (t *Table) setSitStatus() {
	for _, p := range t.seats {
		if p != nil {
			p.SetSit()
		}
	}
}

func (t *Table) onEndTimeout() {
	// 状态进入 StWait
	t.updateStage(StWait)

	// 再次检查踢人
	t.checkKick()

	// 是否自动准备
	t.checkAutoReadyAll()

	// 是否可以下一局
	t.checkCanStart()
}
