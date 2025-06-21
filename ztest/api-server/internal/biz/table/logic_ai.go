package table

import (
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/yola1107/kratos/v2/library/ext"
	"github.com/yola1107/kratos/v2/log"
	v1 "github.com/yola1107/kratos/v2/ztest/api-server/api/helloworld/v1"
	"github.com/yola1107/kratos/v2/ztest/api-server/internal/biz/player"
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
	mTable          *Table
	lastEnterTicker time.Time // 上次进桌时间
	lastExitTicker  time.Time // 上次离桌时间
}

func (r *RobotLogic) init(t *Table) {
	r.mTable = t
}

func (r *RobotLogic) markEnterTime() {
	r.lastEnterTicker = time.Now()
}

func (r *RobotLogic) markExitTime() {
	r.lastExitTicker = time.Now()
}

func (r *RobotLogic) intervalTooShort(last time.Time, minSec, maxSec int) bool {
	return time.Since(last).Seconds() < float64(ext.RandIntInclusive(minSec, maxSec))
}

// CanEnter 判断机器人是否可以进桌
func (r *RobotLogic) CanEnter(p *player.Player) bool {
	cfg := r.mTable.repo.GetRoomConfig().Robot
	if !cfg.Open {
		return false
	}

	// 控制进桌频率
	if p == nil || r.mTable == nil || r.mTable.IsFull() ||
		r.intervalTooShort(r.lastEnterTicker, EnterMinIntervalSec, EnterMaxIntervalSec) {
		return false
	}

	userCnt, aiCnt, _, _ := r.mTable.Counter()
	switch {
	case aiCnt >= cfg.TableMaxCount:
		return false
	case cfg.ReserveN > 0 && r.mTable.ID <= cfg.ReserveN:
		return true
	case userCnt == 0:
		return false
	default:
		return true
	}
}

// CanExit 判断机器人是否可以离开
func (r *RobotLogic) CanExit(p *player.Player) bool {
	cfg := r.mTable.repo.GetRoomConfig().Robot

	// 控制离桌频率
	if p == nil || r.mTable == nil ||
		r.intervalTooShort(r.lastExitTicker, ExitMinIntervalSec, ExitMaxIntervalSec) {
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
		return ext.IsHitFloat(ExitRandChance)
	}
}

func (r *RobotLogic) OnMessage(p *player.Player, cmd v1.GameCommand, msg proto.Message) {
	// 可扩展机器人指令响应逻辑
	if p == nil {
		return
	}

	switch cmd {
	case v1.GameCommand_OnActivePush:
		r.onActivePush(p, msg)
	case v1.GameCommand_OnResultPush:
		r.onExit(p, msg)
	}
}

func (r *RobotLogic) onActivePush(p *player.Player, msg proto.Message) {
	rsp, ok := msg.(*v1.ActivePush)
	if !ok || rsp == nil || p.GetChairID() != rsp.Active {
		return
	}

	ops := rsp.GetCanOp()
	if len(ops) == 0 {
		return
	}

	op := ops[ext.RandIntInclusive(0, len(ops)-1)]
	dur := time.Duration(ext.RandIntInclusive(1, StAction.Timeout())) * time.Second
	log.Debugf("操作列表 ops=%+v, 选中 op=%s, 延迟 dur=%v", ops, op, dur)

	req := &v1.ActionReq{
		UserID:         p.GetPlayerID(),
		Action:         op,
		SideReplyAllow: false,
	}

	r.mTable.repo.GetTimer().Once(dur, func() {
		r.mTable.OnActionReq(p, req, false)
	})
}

func (r *RobotLogic) onExit(p *player.Player, _ proto.Message) {
	if !r.mTable.CanExitRobot(p) {
		return
	}
	r.markExitTime()
	dur := time.Duration(ext.RandInt(2, 6)) * time.Second

	r.mTable.repo.GetTimer().Once(dur, func() {
		r.mTable.OnExitGame(p, 0, "ai exit")
	})
}
