package table

import (
	"sync/atomic"
	"time"

	"github.com/yola1107/kratos/v2/library/xgo"
	"github.com/yola1107/kratos/v2/log"
	v1 "github.com/yola1107/kratos/v2/ztest/game/ludo/api/helloworld/v1"
	"github.com/yola1107/kratos/v2/ztest/game/ludo/internal/biz/player"
	"github.com/yola1107/kratos/v2/ztest/game/ludo/internal/model"
	"google.golang.org/protobuf/proto"
)

const (
	EnterMinIntervalSec = 1
	EnterMaxIntervalSec = 7
	ExitMinIntervalSec  = 3
	ExitMaxIntervalSec  = 10
	ExitRandChance      = 0.05
)

// RobotLogic 封装机器人在桌上的行为逻辑
type RobotLogic struct {
	mTable        *Table
	lastEnterUnix atomic.Int64
	lastExitUnix  atomic.Int64
}

func (r *RobotLogic) init(t *Table) {
	r.mTable = t
}

func (r *RobotLogic) markEnterNow() {
	r.lastEnterUnix.Store(time.Now().Unix())
}

func (r *RobotLogic) markExitNow() {
	r.lastExitUnix.Store(time.Now().Unix())
}

func (r *RobotLogic) EnterTooShort() bool {
	elapsedSec := time.Now().Unix() - r.lastEnterUnix.Load()
	return elapsedSec < int64(xgo.RandIntInclusive(EnterMinIntervalSec, EnterMaxIntervalSec))
}

func (r *RobotLogic) ExitTooShort() bool {
	elapsedSec := time.Now().Unix() - r.lastExitUnix.Load()
	return elapsedSec < int64(xgo.RandIntInclusive(ExitMinIntervalSec, ExitMaxIntervalSec))
}

// CanEnter 判断机器人是否能进桌
func (r *RobotLogic) CanEnter(p *player.Player) bool {
	cfg := r.mTable.repo.GetRoomConfig().Robot
	if !cfg.Open {
		return false
	}

	// 控制进桌频率
	if p == nil || r.mTable == nil || r.mTable.IsFull() || r.EnterTooShort() {
		return false
	}

	// 预留n桌AI自己玩游戏
	n := int32(0)
	if cfg.TableMaxCount > 0 && cfg.MinPlayCount > 0 {
		n = max(1, cfg.MinPlayCount/cfg.TableMaxCount)
	}

	userCnt, aiCnt, _, _ := r.mTable.Counter()
	switch {
	case aiCnt >= cfg.TableMaxCount:
		return false
	case n > 0 && r.mTable.ID <= n:
		return true
	case userCnt == 0:
		return false
	default:
		return true
	}
}

// CanExit 判断机器人是否能离桌
func (r *RobotLogic) CanExit(p *player.Player) bool {
	cfg := r.mTable.repo.GetRoomConfig().Robot
	if !cfg.Open {
		return true
	}
	if p == nil || r.mTable == nil || r.ExitTooShort() {
		return false
	}
	userCnt, aiCnt, _, _ := r.mTable.Counter()
	money := p.GetAllMoney()
	switch {
	case userCnt == 0, aiCnt > cfg.TableMaxCount:
		return true
	case money >= cfg.StandMaxMoney, money <= cfg.StandMinMoney:
		return true
	default:
		return xgo.IsHitFloat(ExitRandChance)
	}
}

// OnMessage AI监听协议消息
func (r *RobotLogic) OnMessage(p *player.Player, cmd v1.GameCommand, msg proto.Message) {
	// 支持自定义 AI 策略插件接口（后续扩展时用）
	if p == nil {
		return
	}
	switch cmd {
	case v1.GameCommand_OnActivePush:
		r.ActivePlayer(p, msg)
	case v1.GameCommand_OnResultPush:
		r.onExit(p, msg)
	default:
		// r.onExit(p, msg) // 测试频繁进退桌 todo delete
	}
}

func (r *RobotLogic) onExit(p *player.Player, _ proto.Message) {
	if !r.mTable.CanExitRobot(p) {
		return
	}
	r.markExitNow() // 记录离桌时间
	dur := time.Duration(xgo.RandInt(ExitMinIntervalSec, ExitMaxIntervalSec)) * time.Second

	r.mTable.repo.GetTimer().Once(dur, func() {
		r.mTable.OnExitGame(p, 0, "ai exit")
	})
}

/*
	AI智能出牌策略
*/

func (r *RobotLogic) ActivePlayer(p *player.Player, msg proto.Message) {
	rsp, ok := msg.(*v1.ActivePush)
	if !ok || rsp == nil || !p.IsGaming() || p.GetChairID() != rsp.Active || p.GetChairID() != r.mTable.active || p.IsFinish() {
		return
	}

	stage := r.mTable.stage.GetState()
	if stage != StMove && stage != StDice {
		return
	}

	if stage == StDice {
		delay := time.Duration(xgo.RandInt(1000, int(r.mTable.stage.Remaining().Milliseconds()/2))) * time.Millisecond
		r.mTable.repo.GetTimer().Once(delay, func() {
			r.mTable.OnDiceReq(p, &v1.DiceReq{Uid: p.GetPlayerID()}, false)
		})
		return
	}

	dices := p.UnusedDice()
	bestId, bestX := model.FindBestMoveSequence(r.mTable.board.Clone(), dices, p.GetColor())
	if bestId <= -1 || bestX <= -1 {
		ret := model.Permute(r.mTable.board.Clone(), p.GetColor(), dices, true)
		log.Errorf("Ai无法移动. 找寻不到路径. tb:%v,p:%v, xdieces=%v, bestId=%v, paths=%v, path2=%v",
			r.mTable.Desc(), p.Desc(), dices, bestId, xgo.ToJSON(p.GetPaths()), xgo.ToJSON(ret))
		return
	}

	delay := time.Duration(xgo.RandInt(1000, int(r.mTable.stage.Remaining().Milliseconds()*3/4))) * time.Millisecond
	r.mTable.repo.GetTimer().Once(delay, func() {
		r.mTable.OnMoveReq(p, &v1.MoveReq{
			UserId:    p.GetPlayerID(),
			PieceId:   bestId,
			DiceValue: bestX,
		}, false)
	})
}
