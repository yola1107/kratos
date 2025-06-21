package table

import (
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/yola1107/kratos/v2/library/ext"
	v1 "github.com/yola1107/kratos/v2/ztest/api-server/api/helloworld/v1"
	"github.com/yola1107/kratos/v2/ztest/api-server/internal/biz/player"
)

/*
	AI机器人游戏逻辑


*/

type RobotLogic struct {
	mTable *Table

	lastEnterTicker time.Time // 玩家进桌时间 （用于Robot）
	lastExitTicker  time.Time // 玩家进桌时间 （用于Robot）
}

func (r *RobotLogic) init(t *Table) {
	r.mTable = t
}

func (r *RobotLogic) updateEnterTicker() {
	r.lastEnterTicker = time.Now()
}

func (r *RobotLogic) updateExitTicker() {
	r.lastExitTicker = time.Now()
}

func (r *RobotLogic) OnMessage(p *player.Player, cmd v1.GameCommand, msg proto.Message) {

}

func (r *RobotLogic) canEnterRobot(p *player.Player) bool {
	if p == nil {
		return false
	}

	if r.mTable.IsFull() {
		return false
	}

	c := r.mTable.repo.GetRoomConfig().Robot
	if !c.Open {
		return false
	}

	// 桌子间隔一定的时间进入AI
	if time.Now().Sub(r.lastEnterTicker).Seconds() < float64(ext.RandInt(1, 7)) {
		return false
	}

	userCnt, aiCnt, _, _ := r.mTable.Counter()

	// 桌子机器人数达到上限不进入桌子
	if aiCnt > c.TableMaxCount {
		return false
	}

	// 预留n桌AI自己玩游戏
	if c.ReserveN > 0 && r.mTable.ID <= c.ReserveN {
		return true
	}

	// 没有真实玩家不进入桌子
	if userCnt == 0 {
		return false
	}

	return true
}

func (r *RobotLogic) canExitRobot(p *player.Player) bool {
	if p == nil {
		return false
	}

	// 桌子间隔一定的时间退出AI
	if time.Now().Sub(r.lastExitTicker).Seconds() < float64(ext.RandInt(3, 7)) {
		return false
	}

	c := r.mTable.repo.GetRoomConfig().Robot

	userCnt, aiCnt, _, _ := r.mTable.Counter()

	// 没有真实玩家离开
	if userCnt == 0 {
		return true
	}

	// 桌子机器人数达到上限 离开
	if aiCnt > c.TableMaxCount {
		return true
	}

	// 金币超过/低于配置 离开
	money := p.GetAllMoney()
	if money >= c.StandMaxMoney || money <= c.StandMinMoney {
		return true
	}

	// 给定一定小概率 离开
	if ext.IsHitFloat(0.05) {
		return true
	}

	return false

}
