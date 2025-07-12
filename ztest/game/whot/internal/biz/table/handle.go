package table

import (
	"github.com/yola1107/kratos/v2/log"
	v1 "github.com/yola1107/kratos/v2/ztest/game/whot/api/helloworld/v1"
	"github.com/yola1107/kratos/v2/ztest/game/whot/internal/biz/player"
	"github.com/yola1107/kratos/v2/ztest/game/whot/pkg/codes"
)

func (t *Table) OnExitGame(p *player.Player, code int32, msg string) bool {
	if !t.ThrowOff(p, false) {
		return false
	}
	t.repo.LogoutGame(p, code, msg)
	return false
}

func (t *Table) OnSceneReq(p *player.Player, isClient bool) {
	t.SendSceneInfo(p)
	return
}

func (t *Table) OnReadyReq(p *player.Player, isReady bool) bool {
	return true
}

func (t *Table) OnChatReq(p *player.Player, in *v1.ChatReq) bool {
	return true
}

func (t *Table) OnHosting(p *player.Player, isHosting bool) bool {
	return true
}

func (t *Table) OnAutoCallReq(p *player.Player, autoCall bool) bool {
	return true
}

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

// OnPlayerActionReq 处理客户端请求的操作
func (t *Table) OnPlayerActionReq(p *player.Player, in *v1.PlayerActionReq, timeout bool) (ok bool) {
	if p == nil || !p.IsGaming() || len(t.GetGamers()) <= 1 || p.GetChairID() != t.active {
		return
	}
	if s := t.stage.GetState(); s == StWait || s == StReady || s == StWaitEnd || s == StEnd {
		return
	}

	// log.Debugf("=> p:%v, ac=%v CanOp=%v, gamer=%+v timeout=%v",
	// 	p.Desc(), in.Action, t.getCanOp(p), len(t.GetGamers()), timeout)

	switch in.Action {
	case v1.ACTION_PLAY_CARD:
		if !t.canOutCard(t.currCard, p.GetCards(), in.OutCard) {
			log.Errorf("playCard err: curr=%v out=%v hand=%v", t.currCard, in.OutCard, p.GetCards())
			return
		}
		t.onPlayCard(p, in.OutCard)

	case v1.ACTION_DRAW_CARD:
		if !t.canDrawCard(p) {
			log.Errorf("drawCard err: 当前不允许摸牌，pending=%+v", t.pending)
			return
		}
		t.onDrawCard(p)

	case v1.ACTION_SKIP_TURN: // 8牌
		if t.pending == nil || t.pending.Effect != v1.CARD_EFFECT_SUSPEND {
			log.Errorf("skipCard err: 当前不允许跳过，pending=%+v", t.pending)
			return
		}
		t.onSkipTurn(p)

	case v1.ACTION_DECLARE_SUIT:
		if t.pending == nil || t.pending.Effect != v1.CARD_EFFECT_WHOT || t.currCard != WhotCard {
			log.Errorf("declareCard err: 当前不允许声明花色, pending=%+v currCard=%d", t.pending, t.currCard)
			return
		}
		if suit := in.DeclareSuit; suit < v1.SUIT_CIRCLE || suit > v1.SUIT_START {
			log.Errorf("declareCard err: 非法花色: %v", in.DeclareSuit)
			return
		}
		t.onDeclareSuit(p, in.DeclareSuit)

	default:
		log.Warnf("未知操作类型: %v", in.Action)
		return
	}

	return true
}

// 处理出牌逻辑
func (t *Table) onPlayCard(p *player.Player, card int32) {
	// 从手牌中移除
	p.RemoveCard(card)

	// 更新当前牌堆顶
	t.currCard = card
	t.declareSuit = v1.SUIT_INVALID

	// 清除 pending 或设置新效果
	t.updatePending(p, card)

	// 广播出牌
	t.broadcastPlayerAction(p, v1.ACTION_PLAY_CARD, []int32{card}, 0)

	// 判断是否出完了
	if len(p.GetCards()) == 0 {
		t.updateStage(StWaitEnd) // 等待结束
		return
	}

	// 14牌：所有其他玩家各抽一张 MARKET, 发牌不够了游戏结束
	if t.pending != nil && t.pending.Effect == v1.CARD_EFFECT_MARKET && Number(card) == 14 {
		if end := t.broadDrawCardByMarket(); end {
			t.updateStage(StWaitEnd)
			return
		}
	}

	log.Debugf("==> playCard. p:%s currCard:%v pending:%v", p.Desc(), t.currCard, t.pending)

	// 通知下个玩家操作
	t.active = t.getNextActiveChair()
	if t.pending != nil {
		t.active = t.pending.Target // 绑定玩家为活动玩家
	}
	t.updateStage(StPlaying)
	t.broadcastActivePlayerPush()
}

func (t *Table) updatePending(p *player.Player, card int32) {
	t.pending = nil
	if !IsSpecialCard(card) {
		return
	}

	nextChair := int32(-1)
	if next := t.NextPlayer(p.GetChairID()); next != nil {
		nextChair = next.GetChairID()
	}

	target := p.GetChairID()
	quantity := int32(1)
	effect := v1.CARD_EFFECT_NORMAL

	switch Number(card) {
	case 1:
		effect = v1.CARD_EFFECT_HOLD_ON
	case 2:
		effect = v1.CARD_EFFECT_PICK_TWO
		target = nextChair
		quantity = 2
	case 8:
		effect = v1.CARD_EFFECT_SUSPEND
		target = nextChair
	case 14:
		effect = v1.CARD_EFFECT_MARKET
	case 20:
		effect = v1.CARD_EFFECT_WHOT
	}

	t.pending = &v1.Pending{
		Initiator: p.GetChairID(),
		Target:    target,
		Effect:    effect,
		Quantity:  quantity,
	}
}

func (t *Table) broadDrawCardByMarket() (end bool) {
	for _, v := range t.seats {
		if v == nil || !v.IsGaming() {
			continue
		}
		drawn := t.cards.DispatchCards(1)
		if len(drawn) == 0 || t.cards.IsEmpty() {
			return true
		}
		t.sendDrawCardPush(v, drawn)
	}
	return false
}

// 处理摸牌逻辑
func (t *Table) onDrawCard(p *player.Player) {
	count := int32(1)
	if t.pending != nil && t.pending.Quantity > 0 {
		count = t.pending.Quantity
	}

	drawn := t.cards.DispatchCards(int(count))
	if len(drawn) == 0 || t.cards.IsEmpty() {
		log.Debugf("无法摸牌（可能牌堆不足），结束游戏")
		t.updateStage(StWaitEnd)
		return
	}

	// 处理pending状态
	if t.pending != nil && t.pending.Target == p.GetChairID() {
		log.Debugf("drawCard. p=%v 响应了 pending，清除: %+v", p.Desc(), t.pending)
		t.pending = nil
	}

	p.AddCards(drawn)
	t.broadcastPlayerAction(p, v1.ACTION_DRAW_CARD, drawn, 0)

	// 通知下个玩家操作
	t.active = t.getNextActiveChair()
	t.updateStage(StPlaying)
	t.broadcastActivePlayerPush()
}

// 处理跳过逻辑
func (t *Table) onSkipTurn(p *player.Player) {
	log.Debugf("skipTurn. p=%v 清除:%+v", p.Desc(), t.pending)
	t.pending = nil

	t.broadcastPlayerAction(p, v1.ACTION_SKIP_TURN, nil, 0)

	// 通知下个玩家操作
	t.active = t.getNextActiveChair()
	t.updateStage(StPlaying)
	t.broadcastActivePlayerPush()
}

// 处理声明花色逻辑
func (t *Table) onDeclareSuit(p *player.Player, suit v1.SUIT) {
	log.Debugf("declareSuit. p=%v suit=%v pending=%+v", p.Desc(), suit, t.pending)
	t.currCard = NewDeclareWhot(int32(suit), t.currCard) // 修改当前牌的花色
	t.declareSuit = suit
	t.pending = nil // 声明后 pending 清除
	t.broadcastPlayerAction(p, v1.ACTION_DECLARE_SUIT, nil, suit)

	// 通知当前玩家操作
	t.active = p.GetChairID()
	t.updateStage(StPlaying)
	t.broadcastActivePlayerPush()
}

// 判断给出的牌是否可以合法出牌
func (t *Table) canOutCard(curr int32, hand []int32, card int32) bool {
	canOuts := calcCanOut(curr, hand)
	for _, c := range canOuts {
		if c == card {
			return true
		}
	}
	return false
}

// 判断是否可以摸牌（根据当前pending效果）
func (t *Table) canDrawCard(p *player.Player) bool {
	pending := t.pending

	if pending != nil && pending.Target != p.GetChairID() {
		log.Errorf("drawCard err: 非目标玩家尝试摸牌: %v", p.Desc())
		return false
	}

	if pending == nil {
		return true
	}
	switch pending.Effect {
	case v1.CARD_EFFECT_NORMAL, v1.CARD_EFFECT_HOLD_ON, v1.CARD_EFFECT_PICK_TWO:
		return true
	default:
		return false
	}
}
