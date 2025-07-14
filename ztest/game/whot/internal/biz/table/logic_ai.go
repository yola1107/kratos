package table

import (
	"math"
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

/*
	AI智能出牌策略
*/
/*
✅ 打出最多牌：优先打掉重复的点数/花色，快速减少手牌
✅ 合理利用当前出牌：优先跟牌，保留未来接牌机会
✅ 控制 Whot：不随意打掉万能牌，留作关键用途
✅ 高度可扩展：后续可添加 AI 模拟、对手预测等
*/

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

	// 对手的手牌数量
	opponentHandSize := r.mTable.GetMinOpponentHandSize(p)
	req := selectBestAction(p, ops, r.mTable.currCard, opponentHandSize)
	if req == nil {
		log.Warnf("no suitable action selected: player=%v table=%v", p.Desc(), r.mTable.Desc())
		return
	}

	delay := time.Duration(ext.RandInt(1000, int(r.mTable.stage.Remaining().Milliseconds()*3/4))) * time.Millisecond
	r.mTable.repo.GetTimer().Once(delay, func() {
		r.mTable.OnPlayerActionReq(p, req, false)
	})
}

// 策略选择
func selectBestAction(p *player.Player, ops []*v1.ActionOption, currCard int32, opponentHandSize int) *v1.PlayerActionReq {
	hand := p.GetCards()
	for _, op := range ops {
		switch op.Action {
		case v1.ACTION_DECLARE_SUIT:
			return &v1.PlayerActionReq{
				UserId:      p.GetPlayerID(),
				Action:      v1.ACTION_DECLARE_SUIT,
				DeclareSuit: getMostFrequentSuit(hand, op.Suits),
			}
		case v1.ACTION_PLAY_CARD:
			if best := chooseBestCard(op.Cards, hand, currCard, opponentHandSize); best > 0 {
				return &v1.PlayerActionReq{
					UserId:  p.GetPlayerID(),
					Action:  v1.ACTION_PLAY_CARD,
					OutCard: best,
				}
			}
		case v1.ACTION_DRAW_CARD:
			return &v1.PlayerActionReq{
				UserId: p.GetPlayerID(),
				Action: v1.ACTION_DRAW_CARD,
			}
		case v1.ACTION_SKIP_TURN:
			return &v1.PlayerActionReq{
				UserId: p.GetPlayerID(),
				Action: v1.ACTION_SKIP_TURN,
			}
		}
	}
	return nil
}

// 出最多的花色
func getMostFrequentSuit(hand []int32, options []v1.SUIT) v1.SUIT {
	suitCount := make(map[v1.SUIT]int)
	for _, c := range hand {
		suitCount[v1.SUIT(Suit(c))]++
	}

	var bestSuit v1.SUIT
	highest := -1
	for _, s := range options {
		if suitCount[s] > highest {
			bestSuit = s
			highest = suitCount[s]
		}
	}
	return bestSuit
}

// 出牌选择策略
func chooseBestCard(candidates, hand []int32, currCard int32, opponentHandSize int) int32 {
	if len(candidates) == 0 {
		return 0
	}

	currSuit := v1.SUIT(Suit(currCard))
	currNum := Number(currCard)

	var bestCard int32
	bestScore := math.MinInt

	for _, c := range candidates {
		score := evaluateCardScoreV2(c, currSuit, currNum, hand, opponentHandSize)
		if score > bestScore {
			bestCard = c
			bestScore = score
		}
	}
	return bestCard
}

// 优化版评分函数
func evaluateCardScoreV2(card int32, currSuit v1.SUIT, currNum int32, hand []int32, opponentHandSize int) int {
	if IsWhotCard(card) {
		switch {
		case len(hand) == 1:
			return 100
		case len(hand) <= 3:
			return 40
		default:
			return -300
		}
	}

	s := v1.SUIT(Suit(card))
	n := Number(card)

	numCount := make(map[int32]int)
	suitCount := make(map[v1.SUIT]int)
	for _, c := range hand {
		numCount[Number(c)]++
		suitCount[v1.SUIT(Suit(c))]++
	}

	score := 0

	if s == currSuit {
		score += 10
	}
	if n == currNum {
		score += 10
	}

	score -= (3 - numCount[n]) * 4
	score -= (2 - suitCount[s]) * 3

	switch n {
	case 2:
		if opponentHandSize > 2 {
			score += 10
		} else {
			score -= 5
		}
	case 8:
		score += 5
	case 14:
		score += 12
	case 20:
		if opponentHandSize > 1 {
			score += 10
		} else {
			score -= 10
		}
	}

	canChain := false
	for _, c := range hand {
		if c == card {
			continue
		}
		if Suit(c) == int32(s) || Number(c) == n {
			canChain = true
			break
		}
	}
	if canChain {
		score += 5
	}

	if len(hand) <= 3 {
		score -= (3 - numCount[n]) * 6
		score -= (2 - suitCount[s]) * 5
	}

	return score
}

func (t *Table) GetMinOpponentHandSize(p *player.Player) int {
	opponentHandSize := 54
	if p == nil {
		return opponentHandSize
	}
	for _, v := range t.seats {
		if v == nil || !v.IsGaming() || v.GetChairID() == p.GetChairID() {
			continue
		}
		if len(v.GetCards()) < opponentHandSize {
			opponentHandSize = len(v.GetCards())
		}
	}
	return opponentHandSize
}

/*
	AI智能摸牌,剩余牌堆中选对自己有利的牌
*/

// DrawSmartCard 智能摸牌 DrawSmartCard
func (t *Table) DrawSmartCard(p *player.Player) int32 {
	heap := t.cards.GetCards()
	if len(heap) == 0 {
		return 0
	}

	hand := p.GetCards()
	currCard := t.currCard
	currSuit := v1.SUIT(Suit(currCard))
	currNum := Number(currCard)

	// 如果不希望暴露对手数量，可以固定给个值
	opponentHandSize := t.GetMinOpponentHandSize(p)

	var bestCard int32
	bestScore := math.MinInt
	bestIdx := -1

	for i, c := range heap {
		score := evaluateDrawCardScore(c, currSuit, currNum, hand, opponentHandSize)
		if score > bestScore {
			bestScore = score
			bestCard = c
			bestIdx = i
		}
	}

	if bestIdx >= 0 {
		// 注意：真实抽牌逻辑应该在发牌系统中执行；这里只是选择用哪张而已
		return bestCard
	}
	return 0
}

// 摸牌专用评分：更倾向于补充连号、同花色，不太倾向功能牌
func evaluateDrawCardScore(card int32, currSuit v1.SUIT, currNum int32, hand []int32, opponentHandSize int) int {
	if IsWhotCard(card) {
		return -100 // 摸牌阶段避免抽Whot
	}

	s := v1.SUIT(Suit(card))
	n := Number(card)

	numCount := make(map[int32]int)
	suitCount := make(map[v1.SUIT]int)
	for _, c := range hand {
		numCount[Number(c)]++
		suitCount[v1.SUIT(Suit(c))]++
	}

	score := 0

	// 如果和当前场面牌相符，得分
	if s == currSuit {
		score += 8
	}
	if n == currNum {
		score += 8
	}

	// 补充已有的花色/数字
	score += numCount[n] * 3
	score += suitCount[s] * 2

	// 功能牌轻微加分（可策略调节）
	if n == 2 || n == 14 || n == 20 {
		score += 2
	}

	return score
}
