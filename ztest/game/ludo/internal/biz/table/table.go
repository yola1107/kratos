package table

import (
	"fmt"
	"time"

	"github.com/yola1107/kratos/v2/log"
	"github.com/yola1107/kratos/v2/ztest/game/ludo/internal/biz/player"
	"github.com/yola1107/kratos/v2/ztest/game/ludo/internal/conf"
	"github.com/yola1107/kratos/v2/ztest/game/ludo/internal/model"
	"github.com/yola1107/kratos/v2/ztest/game/ludo/pkg/codes"
)

type Table struct {
	ID       int32 // 桌子ID
	Type     TYPE  // 类型
	MaxCnt   int16 // 最大玩家数
	isClosed bool  // 是否停服
	repo     Repo  //

	// 游戏变量
	stage   *Stage           // 阶段状态
	mLog    *Log             // 桌子日志
	seats   []*player.Player // 玩家列表
	sitCnt  int16            // 入座玩家数量
	active  int32            // 当前操作玩家
	first   int32            // 第一个操作玩家
	aiLogic RobotLogic       // 机器人逻辑

	board        *model.Board // 游戏棋盘
	timerJobFast *time.Timer  // 快速场结算定时器
}

func NewTable(id int32, typ TYPE, c *conf.Room, repo Repo) *Table {
	t := &Table{
		ID:           id,
		Type:         typ,
		MaxCnt:       int16(c.Table.ChairNum),
		repo:         repo,
		stage:        &Stage{},
		sitCnt:       0,
		active:       -1,
		first:        -1,
		mLog:         NewTableLog(id, c.LogCache),
		seats:        make([]*player.Player, c.Table.ChairNum),
		board:        nil,
		timerJobFast: nil,
	}
	t.aiLogic.init(t)
	return t
}

func (t *Table) Reset() {
	t.active = -1
	t.first = -1
	t.board.Clear()
	t.board = nil

	for _, seat := range t.seats {
		if seat == nil {
			continue
		}
		seat.Reset()
	}
}

func (t *Table) Desc() string {
	str := fmt.Sprintf("(T:%d SitCnt:%d Gamers:%d St:%+v First:%d active:%d )",
		t.ID, t.sitCnt, len(t.GetGamers()), t.stage.GetState(), t.first, t.active)
	return str
}

func (t *Table) Empty() bool {
	return t.sitCnt <= 0
}

func (t *Table) IsFull() bool {
	return t.sitCnt >= t.MaxCnt
}

func (t *Table) GetSitCnt() int32 {
	return int32(t.sitCnt)
}

// ThrowInto 入座
func (t *Table) ThrowInto(p *player.Player) bool {
	for k, v := range t.seats {
		if v != nil {
			continue
		}

		// 桌子信息
		t.seats[k] = p
		t.sitCnt++

		// 玩家信息
		p.Reset()
		p.SetTableID(t.ID)
		p.SetChairID(int32(k))
		p.SetSit()
		t.checkAutoReady(p)

		// 通知客户端登录成功
		t.SendLoginRsp(p, codes.SUCCESS, "")

		// 广播入座信息
		t.broadcastUserInfo(p)

		// 发送场景信息
		t.SendSceneInfo(p)

		// 记录进桌时间
		t.aiLogic.markEnterNow()

		// 日志记录
		t.mLog.userEnter(p, t.sitCnt)
		log.Infof("EnterTable. p:%+v sitCnt:%d", p.Desc(), t.sitCnt)

		// 检查是否可开局
		t.checkCanStart()

		// 上报桌子/玩家位置 todo
		return true
	}
	return false
}

// ThrowOff 出座
func (t *Table) ThrowOff(p *player.Player, isSwitchTable bool) bool {
	if p == nil {
		return false
	}

	chair := p.GetChairID()
	if p1 := t.GetPlayerByChair(chair); p1 != p {
		return false
	}

	if !t.CanExit(p) {
		return false
	}

	t.seats[p.GetChairID()] = nil
	t.sitCnt--

	// 广播玩家离桌
	t.broadcastUserQuitPush(p, isSwitchTable)

	// 记录时间
	t.aiLogic.markExitNow()

	// 重置玩家信息
	p.ExitReset()

	// 上报桌子/玩家位置 todo
	t.mLog.userExit(p, t.sitCnt, chair, isSwitchTable)
	log.Infof("ExitTable. p:%+v sitCnt:%d st:%v isSwitch:%+v", p.Desc(), t.sitCnt, t.stage.GetState(), isSwitchTable)
	return true
}

// ReEnter 重进游戏
func (t *Table) ReEnter(p *player.Player) {
	// 通知客户端登录成功
	t.SendLoginRsp(p, codes.SUCCESS, "ReEnter")

	// 广播入座信息
	t.broadcastUserInfo(p)

	// 发送场景信息
	t.SendSceneInfo(p)

	p.SetOffline(false) // 是否需要广播状态？

	t.broadcastUserOffline(p)

	t.mLog.userReEnter(p, t.sitCnt)
	log.Infof("ReEnterTable. p:%+v sitCnt:%d", p.Desc(), t.sitCnt)
}

func (t *Table) CanEnter(p *player.Player) bool {
	return p != nil && !t.IsFull()
}

func (t *Table) CanExit(p *player.Player) bool {
	s := t.stage.GetState()
	return p != nil && !p.IsGaming() && (s == StWait || s == StResult)
}

func (t *Table) CanSwitchTable(p *player.Player) bool {
	return t.CanExit(p)
}

func (t *Table) CanEnterRobot(p *player.Player) bool {
	return t.CanEnter(p) && t.aiLogic.CanEnter(p)
}

func (t *Table) CanExitRobot(p *player.Player) bool {
	return t.CanExit(p) && t.aiLogic.CanExit(p)
}

// LastPlayer 上一家
func (t *Table) LastPlayer(chair int32) *player.Player {
	maxCnt := int32(t.MaxCnt)
	for i := int32(0); i < maxCnt; i++ {
		chair--
		if chair < 0 {
			chair = maxCnt - 1
		}
		if t.seats[chair] == nil || !t.seats[chair].IsGaming() || t.seats[chair].IsFinish() {
			continue
		}
		return t.seats[chair]
	}
	return nil
}

// NextPlayer 轮流寻找玩家
func (t *Table) NextPlayer(chair int32) *player.Player {
	maxCnt := int32(t.MaxCnt)
	for i := int32(0); i < maxCnt; i++ {
		chair = (chair + 1) % maxCnt
		if t.seats[chair] == nil || !t.seats[chair].IsGaming() || t.seats[chair].IsFinish() {
			continue
		}
		return t.seats[chair]
	}

	return nil
}

func (t *Table) GetActivePlayer() *player.Player {
	active := t.active
	if active < 0 || active >= int32(t.MaxCnt) {
		return nil
	}
	return t.seats[active]
}

func (t *Table) GetNextActivePlayer() *player.Player {
	if t.active < 0 || t.active >= int32(t.MaxCnt) {
		return nil
	}
	return t.NextPlayer(t.active)
}

func (t *Table) getNextActiveChair() int32 {
	p := t.GetNextActivePlayer()
	if p == nil {
		log.Errorf("getNextActivePlayerChair: nil p. active=%+v", t.active)
		return -1 // 容错
	}
	return p.GetChairID()
}

func (t *Table) GetPlayerByChair(chair int32) *player.Player {
	if chair < 0 || chair >= int32(t.MaxCnt) {
		return nil
	}
	return t.seats[chair]
}

// GetGamers 返回“仍可继续操作”的玩家：
//  1. 仍在本局游戏中 (IsGaming)
//  2. 有可以活动棋子(未到终点)
func (t *Table) GetGamers() (seats []*player.Player) {
	for _, p := range t.seats {
		if p == nil || !p.IsGaming() || p.IsFinish() {
			continue
		}
		seats = append(seats, p)
	}
	return seats
}

func (t *Table) Counter() (userCnt, aiCnt, allCnt, gamingCnt int32) {
	for _, seat := range t.seats {
		if seat == nil {
			continue
		}
		if seat.IsRobot() {
			aiCnt++
		} else {
			userCnt++
		}
		if seat.IsGaming() {
			gamingCnt++
		}
		allCnt++
	}
	return
}

func (t *Table) checkKick() {
	for _, p := range t.seats {
		if p == nil {
			continue
		}
		if code, msg := t.checkKickPlayer(p, t.repo.GetRoomConfig().Game); code != 0 {
			t.OnExitGame(p, code, msg)
		}
	}
}

func (t *Table) checkKickPlayer(p *player.Player, conf *conf.Room_Game) (int32, string) {
	switch t.stage.GetState() {
	case StWait:
		if p.IsOffline() {
			return codes.KICK_BY_BROKE, "KICK_BY_BROKE"
		}
		if code, msg := CheckRoomLimit(p, conf); code != 0 {
			return code, msg
		}
	case StResult:
		if p.IsOffline() {
			return codes.KICK_BY_BROKE, "KICK_BY_BROKE"
		}
	default:
	}
	return 0, ""
}
