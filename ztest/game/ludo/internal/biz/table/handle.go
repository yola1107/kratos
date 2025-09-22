package table

import (
	"github.com/yola1107/kratos/v2/library/xgo"
	"github.com/yola1107/kratos/v2/log"
	v1 "github.com/yola1107/kratos/v2/ztest/game/ludo/api/helloworld/v1"
	"github.com/yola1107/kratos/v2/ztest/game/ludo/internal/biz/player"
	"github.com/yola1107/kratos/v2/ztest/game/ludo/internal/model"
	"github.com/yola1107/kratos/v2/ztest/game/ludo/pkg/codes"
)

// OnExitGame 玩家请求退出游戏
func (t *Table) OnExitGame(p *player.Player, code int32, msg string) bool {
	if !t.ThrowOff(p, false) {
		return false
	}
	t.repo.LogoutGame(p, code, msg)
	return false
}

// OnSceneReq 请求场景信息（重连时调用）
func (t *Table) OnSceneReq(p *player.Player, _ bool) {
	t.SendSceneInfo(p)
}

func (t *Table) OnReadyReq(*player.Player, bool) bool       { return true }
func (t *Table) OnChatReq(*player.Player, *v1.ChatReq) bool { return true }
func (t *Table) OnHosting(*player.Player, bool) bool        { return true }
func (t *Table) OnAutoCallReq(*player.Player, bool) bool    { return true }

// OnOffline 玩家断线处理
func (t *Table) OnOffline(p *player.Player) bool {
	t.mLog.offline(p)
	if !p.IsGaming() {
		t.OnExitGame(p, codes.KICK_BY_BROKE, "OnOffline kick by broke")
		return true
	}
	p.SetOffline(true)
	t.broadcastUserOffline(p)
	return true
}

// OnDiceReq 玩家请求掷骰子
func (t *Table) OnDiceReq(p *player.Player, in *v1.DiceReq, timeout bool) bool {
	if p == nil || t.stage.GetState() != StDice || p.GetChairID() != t.active || !p.IsGaming() || p.IsFinish() {
		return false
	}

	// 执行掷骰逻辑
	dice := t.ctrlRollDice(p)   // 生成骰子点数
	p.AddDice(dice)             // 加入到玩家骰子列表
	p.IncrTimeoutCnt(timeout)   // 超时计数
	t.broadcastDiceRsp(p, dice) // 广播掷骰结果

	movable := t.hasMovableOption(p)          // 是否存在可动棋子
	tripleSix := dice == 6 && p.IsTripleSix() // 是否连续掷出三个6

	t.mLog.Dice(p, dice, movable, tripleSix)
	log.Debugf("OnDiceReq: p=%v, dice=%d, movable=%v, tripleSix=%v, timeout=%v",
		p.Desc(), dice, movable, tripleSix, timeout)

	// 回合控制
	switch {
	case (!movable) || tripleSix:
		// 无法移动 或 连续三个6，直接结束本轮
		t.endPlayerTurn(p)
	case dice == 6:
		// 奖励再掷一次骰子
		t.repeatPlayerTurn(p)
	default:
		// 允许进入移动阶段
		t.allowPlayerToMove(p)
	}
	return true
}

// OnMoveReq 玩家请求移动棋子
func (t *Table) OnMoveReq(p *player.Player, in *v1.MoveReq, timeout bool) bool {
	if p == nil || t.stage.GetState() != StMove || p.GetChairID() != t.active || !p.IsGaming() || p.IsFinish() {
		return false
	}
	if !p.HasUnusedDice(in.DiceValue) {
		log.Errorf("OnMoveReq failed: unused dice mismatch. p=%v, in=%v, code=%d", p.Desc(), in, model.ErrDiceMismatch)
		return false
	}
	if ok, code := t.board.CanMove(p.GetColor(), in.GetPieceId(), in.DiceValue); !ok {
		log.Errorf("OnMoveReq failed: move not allowed. p=%v, in=%v, code=%d", p.Desc(), in, code)
		return false
	}

	// 执行移动逻辑
	pieceID, dice := in.PieceId, in.DiceValue
	step := t.board.Move(pieceID, dice)             // 实际移动
	piece := t.board.GetPieceByID(pieceID)          // 获取棋子
	arrived := piece.IsArrived()                    // 棋子是否到达终点
	p.UseDice(dice)                                 // 消耗骰子点数
	p.IncrTimeoutCnt(timeout)                       // 超时计数
	t.broadcastMoveRsp(p, pieceID, dice, nil, step) // 广播移动结果

	t.mLog.Move(p, pieceID, dice, arrived, step, timeout)
	log.Debugf("OnMoveReq. p=%v, req={Id:%d,X:%d} step=%v, timeout=%v",
		p.Desc(), pieceID, dice, xgo.ToJSON(step), timeout)

	// 到达终点处理
	if arrived {
		p.MarkPieceArrived(pieceID)
		t.checkPlayerFinish(p)
		log.Debugf("===> 棋子进入终点. p=%v, id=%d, x=%d, finish=%v", p.Desc(), pieceID, dice, p.IsFinish())

		// 是否结束游戏
		if p.IsFinish() {
			t.settle(0)
			return true
		}
	}

	// 回合控制
	switch {
	case p.IsFinish():
		// 玩家完成所有棋子，直接结束回合
		t.endPlayerTurn(p)
	case arrived || len(step.Killed) > 0:
		// 踩子 或 到达终点, 奖励再掷一次
		t.repeatPlayerTurn(p)
	case !t.hasMovableOption(p):
		// 没有可移动棋子，回合结束
		t.endPlayerTurn(p)
	default:
		// 还有可用骰子继续移动
		t.allowPlayerToMove(p)
	}
	return true
}

// // CalcFastModeScore Fast 模式下计分逻辑
// func (t *Table) CalcFastModeScore(color int32, step *model.Step, arrived bool) map[int32]int64 {
// 	deltaMap := make(map[int32]int64)
// 	if step == nil || step.From == step.To {
// 		return deltaMap
// 	}
//
// 	deltaMap[color] += int64(step.X)
// 	if arrived {
// 		deltaMap[color] += 50 // 到达终点奖励50分
// 	}
//
// 	for _, v := range step.Killed {
// 		if killed := t.board.GetPieceByID(v.Id); killed != nil {
// 			dis := model.StepsFromStart(v.From, killed.Color())
// 			deltaMap[killed.Color()] -= int64(dis) // 被吃棋子减掉移动的步数
// 			deltaMap[color] += 20                  // 吃一颗棋子奖励20分
// 		}
// 	}
//
// 	// 加减分
// 	for k, v := range deltaMap {
// 		t.fastScore[k] += v
// 	}
// 	return deltaMap
// }

// 检查是否玩家棋子已全部到达终点
func (t *Table) checkGameOver() bool {
	maxCnt := int(t.MaxCnt)
	fleeCnt := 0 // len(t.fleeUsers)
	finishCnt := 0

	for _, p := range t.seats {
		if p != nil && p.IsFinish() {
			finishCnt++
		}
	}

	if fleeCnt > 0 {
		return finishCnt+fleeCnt >= maxCnt-1
	}
	return finishCnt >= maxCnt/2
}

// 是否所有棋子都进入终点
func (t *Table) checkPlayerFinish(p *player.Player) {
	if len(t.board.GetActivePieceIDs(p.GetColor())) == 0 {
		p.SetFinish()
	}
}

// 判断玩家是否还有未使用的色子能继续移动
func (t *Table) hasMovableOption(p *player.Player) bool {
	return t.board.CalcCanMoveDice(p.GetColor(), p.UnusedDice())
}

// 设置玩家为当前行动者，并切换至“移动”阶段
func (t *Table) allowPlayerToMove(p *player.Player) {
	t.active = p.GetChairID()
	t.updateStage(StMove)
	t.broadcastActivePlayerPush()
}

// 设置玩家为当前行动者，并切换至“掷骰”阶段（奖励回合）
func (t *Table) repeatPlayerTurn(p *player.Player) {
	t.active = p.GetChairID()
	t.updateStage(StDice)
	t.broadcastActivePlayerPush()
}

// 玩家回合结束，清空色子，切换至下一玩家掷骰
func (t *Table) endPlayerTurn(p *player.Player) {
	p.FinishTurn()
	t.active = t.getNextActiveChair()
	t.updateStage(StDice)
	t.broadcastActivePlayerPush()
}

func (t *Table) ctrlRollDice(p *player.Player) int32 {
	// 6点保护策略 (非快速模式。 前 n 次必须至少出过一次6)
	if x := p.RollInitDiceList(); x >= 1 && x <= 6 {
		return x
	}
	// 避免频繁连续6点 66 666
	if p.GetLastRoll() == 6 && xgo.IsHitFloat(0.9) {
		return xgo.RandInt[int32](1, 6)
	}
	// 控制数量
	if len(p.GetDiceSlot()) >= 4 {
		return xgo.RandInt[int32](1, 6)
	}
	return xgo.RandInt[int32](1, 7)
}
