package table

import (
	"sync/atomic"
	"time"

	"github.com/yola1107/kratos/v2/library/ext"
	"github.com/yola1107/kratos/v2/log"
	v1 "github.com/yola1107/kratos/v2/ztest/game/whot/api/helloworld/v1"
	"github.com/yola1107/kratos/v2/ztest/game/whot/internal/biz/player"
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
	return elapsedSec < int64(ext.RandIntInclusive(EnterMinIntervalSec, EnterMaxIntervalSec))
}

func (r *RobotLogic) ExitTooShort() bool {
	elapsedSec := time.Now().Unix() - r.lastExitUnix.Load()
	return elapsedSec < int64(ext.RandIntInclusive(ExitMinIntervalSec, ExitMaxIntervalSec))
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
		r.ActivePlayer(p, msg)
	case v1.GameCommand_OnResultPush:
		r.onExit(p, msg)
	default:
		r.onExit(p, msg) // 测试频繁进退桌 todo delete
	}
}

func (r *RobotLogic) ActivePlayer(p *player.Player, msg proto.Message) {
	rsp, ok := msg.(*v1.ActivePush)
	if !ok || rsp == nil || !p.IsGaming() || p.GetChairID() != rsp.Active || p.GetChairID() != r.mTable.active {
		return
	}

	ops := r.mTable.getCanOp(p)
	if len(ops) == 0 {
		log.Errorf("no available options: player=%v table=%v", p.Desc(), r.mTable.Desc())
		return
	}

	log.Debugf("=> p:%v, curr=%v, canOps=%v", p.Desc(), r.mTable.currCard, ext.ToJSON(ops))

	req := &v1.PlayerActionReq{UserId: p.GetPlayerID()}

pick:
	for _, op := range ops {
		switch op.Action {
		case v1.ACTION_PLAY_CARD:
			var nonWhot []int32
			for _, c := range op.Cards {
				if IsWhotCard(c) {
					nonWhot = append(nonWhot, c)
				}
			}
			if len(nonWhot) > 0 {
				req.Action = v1.ACTION_PLAY_CARD
				req.OutCard = nonWhot[ext.RandInt(0, len(nonWhot))]
				break pick
			}
			if len(op.Cards) > 0 {
				req.Action = v1.ACTION_PLAY_CARD
				req.OutCard = op.Cards[ext.RandInt(0, len(op.Cards))]
				break pick
			}

		case v1.ACTION_DRAW_CARD:
			if req.Action == 0 {
				req.Action = v1.ACTION_DRAW_CARD
			}

		case v1.ACTION_SKIP_TURN:
			if req.Action == 0 {
				req.Action = v1.ACTION_SKIP_TURN
			}

		case v1.ACTION_DECLARE_SUIT:
			req.Action = v1.ACTION_DECLARE_SUIT
			if len(op.Suits) > 0 {
				req.DeclareSuit = op.Suits[ext.RandInt(0, len(op.Suits))]
			}

		default:
			log.Warnf("unknown action=%v for player=%v", op.Action, p.Desc())
		}
	}

	if req.Action == 0 {
		log.Warnf("no suitable action selected for player=%v at table=%v", p.Desc(), r.mTable.Desc())
		return
	}

	remaining := r.mTable.stage.Remaining().Milliseconds()
	delay := time.Duration(ext.RandInt(800, int(remaining*3/4))) * time.Millisecond
	r.mTable.repo.GetTimer().Once(delay, func() {
		r.mTable.OnPlayerActionReq(p, req, false)
	})
}

func (r *RobotLogic) onExit(p *player.Player, _ proto.Message) {
	if !r.mTable.CanExitRobot(p) {
		return
	}
	r.markExitNow() // 记录离桌时间
	dur := time.Duration(ext.RandInt(ExitMinIntervalSec, ExitMaxIntervalSec)) * time.Second

	r.mTable.repo.GetTimer().Once(dur, func() {
		r.mTable.OnExitGame(p, 0, "ai exit")
	})
}
