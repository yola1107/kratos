package table

import (
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/yola1107/kratos/v2/library/ext"
	"github.com/yola1107/kratos/v2/log"
	v1 "github.com/yola1107/kratos/v2/ztest/api-server/api/helloworld/v1"
	"github.com/yola1107/kratos/v2/ztest/api-server/internal/biz/player"
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

// CanEnter 判断机器人是否可以进桌
func (r *RobotLogic) CanEnter(p *player.Player) bool {
	if p == nil || r.mTable == nil || r.mTable.IsFull() {
		return false
	}

	cfg := r.mTable.repo.GetRoomConfig().Robot
	if !cfg.Open {
		return false
	}

	// 控制进桌频率
	if time.Since(r.lastEnterTicker).Seconds() < float64(ext.RandInt(1, 7)) {
		return false
	}

	userCnt, aiCnt, _, _ := r.mTable.Counter()

	// AI数达到最大上限
	if aiCnt >= cfg.TableMaxCount {
		return false
	}

	// 保留前N桌给AI玩
	if cfg.ReserveN > 0 && r.mTable.ID <= cfg.ReserveN {
		return true
	}

	// 没有真人玩家就不进桌
	if userCnt == 0 {
		return false
	}

	return true
}

// CanExit 判断机器人是否可以离开
func (r *RobotLogic) CanExit(p *player.Player) bool {
	if p == nil || r.mTable == nil {
		return false
	}

	cfg := r.mTable.repo.GetRoomConfig().Robot

	// 控制离桌频率
	if time.Since(r.lastExitTicker).Seconds() < float64(ext.RandInt(3, 10)) {
		return false
	}

	userCnt, aiCnt, _, _ := r.mTable.Counter()

	if userCnt == 0 || aiCnt > cfg.TableMaxCount {
		return true
	}

	// 金币超过/低于配置 离开
	money := p.GetAllMoney()
	if money >= cfg.StandMaxMoney || money <= cfg.StandMinMoney {
		return true
	}

	// 小概率自动离开
	return ext.IsHitFloat(0.05)
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
	if p == nil {
		return
	}

	rsp, ok := msg.(*v1.ActivePush)
	if !ok || rsp == nil {
		return
	}

	if p.GetChairID() != rsp.Active {
		return
	}

	if ops := rsp.GetCanOp(); len(ops) > 0 {
		op := ops[ext.RandInt(0, len(ops)-1)]
		dur := time.Duration(ext.RandInt(1, StActionTimeout-1)) * time.Second
		req := &v1.ActionReq{
			UserID:         p.GetPlayerID(),
			Action:         op,
			SideReplyAllow: false,
		}
		log.Debugf("ops=[%+v] op:%s dur=%v", descActions(ops...), descActions(op), dur)
		r.mTable.repo.GetTimer().Once(dur, func() {
			r.mTable.OnActionReq(p, req, false)
		})
	}
}

func (r *RobotLogic) onExit(p *player.Player, _ proto.Message) {
	if p == nil {
		return
	}
	if !r.mTable.CanExitRobot(p) {
		return
	}
	r.markExitTime()
	dur := time.Duration(ext.RandInt(2, 6))
	log.Debugf("[RobotExit] table:%d uid:%d delay:%ds", r.mTable.ID, p.GetPlayerID(), dur)
	r.mTable.repo.GetTimer().Once(dur, func() {
		r.mTable.OnExitGame(p, 0, "ai exit")
	})
}
