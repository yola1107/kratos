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
	lastEnterTicker time.Time
	lastExitTicker  time.Time
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

// 判断间隔是否太短
func (r *RobotLogic) intervalTooShort(last time.Time, minSec, maxSec int) bool {
	randDur := time.Duration(ext.RandIntInclusive(minSec, maxSec)) * time.Second
	return time.Since(last) < randDur
}

// CanEnter 判断机器人是否能进桌
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

	// 预留n桌AI自己玩游戏
	n := int32(0)
	if cfg.TableMaxCount > 0 && cfg.MinPlayCount > 0 {
		n = max(1, cfg.MinPlayCount/cfg.TableMaxCount)
	}

	userCnt, aiCnt, _, _ := r.mTable.Counter()
	switch {
	case aiCnt >= cfg.TableMaxCount:
		return false
		// case cfg.ReserveN > 0 && r.mTable.ID <= cfg.ReserveN:
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

// OnMessage AI监听协议消息
func (r *RobotLogic) OnMessage(p *player.Player, cmd v1.GameCommand, msg proto.Message) {
	// 支持自定义 AI 策略插件接口（后续扩展时用）
	if p == nil {
		return
	}
	switch cmd {
	case v1.GameCommand_OnActivePush:
		r.onActivePush(p, msg)
	case v1.GameCommand_OnResultPush:
		r.onExit(p, msg)
	default:
		// 测试频繁进退桌 todo del
		r.onExit(p, msg)
		// 测试自动看牌功能
		if ext.IsHitFloat(0.02) {
			r.onRandSee(p, msg)
		}
	}
}

func (r *RobotLogic) onActivePush(p *player.Player, msg proto.Message) {
	rsp, ok := msg.(*v1.ActivePush)
	if !ok || rsp == nil || p.GetChairID() != rsp.Active {
		return
	}

	ops := rsp.GetCanOp()
	ops2 := r.mTable.getCanOp(r.mTable.GetActivePlayer())
	if !(ext.SliceContains(ops, ops2...) && ext.SliceContains(ops2, ops...) && len(ops2)*len(ops) != 0) {
		log.Errorf("ops:%v ops2:%v ", ops, ops2)
	}

	if len(ops) == 0 {
		log.Errorf("RobotLogic.onActivePush: no action found in active push. ops:%v ops2:%v ", ops, ops2)
		ops = ops2
	}

	op := RandOpWithWeight(ops)                            // 按权重随机选操作
	remaining := r.mTable.stage.Remaining().Milliseconds() // 获取剩余操作时间
	dur := time.Duration(ext.RandInt(1000, remaining-1000)) * time.Millisecond
	// log.Debugf("p:%v CanOp=%+v, OP=%s, dur=%v", p.Desc(), ops, op, dur)

	req := &v1.ActionReq{
		UserID:         p.GetPlayerID(),
		Action:         op,
		SideReplyAllow: ext.IsHitFloat(0.5),
	}

	r.mTable.repo.GetTimer().Once(dur, func() {
		r.mTable.OnActionReq(p, req, false)
	})
}

func (r *RobotLogic) onExit(p *player.Player, _ proto.Message) {
	if !r.mTable.CanExitRobot(p) {
		return
	}
	r.markExitTime() // 记录离桌时间
	dur := time.Duration(ext.RandInt(ExitMinIntervalSec, ExitMaxIntervalSec)) * time.Second

	r.mTable.repo.GetTimer().Once(dur, func() {
		r.mTable.OnExitGame(p, 0, "ai exit")
	})
}

func (r *RobotLogic) onRandSee(_ *player.Player, _ proto.Message) {
	canSeenSeats := []*player.Player(nil)
	for _, v := range r.mTable.seats {
		if v == nil || !v.IsGaming() || v.Seen() {
			continue
		}
		canSeenSeats = append(canSeenSeats, v)
	}
	if len(canSeenSeats) == 0 {
		return
	}
	see := canSeenSeats[ext.RandInt(0, len(canSeenSeats))]
	dur := time.Duration(ext.RandInt(3, 7)) * time.Second
	r.mTable.repo.GetTimer().Once(dur, func() {
		r.mTable.OnActionReq(see, &v1.ActionReq{Action: v1.ACTION_SEE}, false)
	})
	// log.Debugf("seen by self. p:%v  dur=%v", see.Desc(), dur)
}

// RandOpWithWeight 按权重从ops中随机选择一个动作
func RandOpWithWeight(ops []v1.ACTION) v1.ACTION {
	if len(ops) == 0 {
		return -1
	}
	weights := map[v1.ACTION]int{
		v1.ACTION_SEE:        5,
		v1.ACTION_CALL:       4,
		v1.ACTION_RAISE:      3,
		v1.ACTION_SHOW:       2,
		v1.ACTION_SIDE:       5,
		v1.ACTION_SIDE_REPLY: 3,
		v1.ACTION_PACK:       1,
	}

	pool := make([]v1.ACTION, 0, len(ops)*10)
	for _, op := range ops {
		w := weights[op]
		if w == 0 {
			w = 1 // 默认权重1
		}
		for i := 0; i < w; i++ {
			pool = append(pool, op)
		}
	}

	if len(pool) == 0 {
		log.Warnf("RandOpWithWeight empty pool, fallback to first op")
		return ops[0]
	}

	return pool[ext.RandIntInclusive(0, len(pool)-1)]
}
