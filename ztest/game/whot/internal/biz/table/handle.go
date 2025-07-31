package table

import (
	"fmt"

	"github.com/yola1107/kratos/v2/library/xgo"
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

func (t *Table) OnSceneReq(p *player.Player, _ bool) { t.SendSceneInfo(p) }

func (t *Table) OnReadyReq(*player.Player, bool) bool       { return true }
func (t *Table) OnChatReq(*player.Player, *v1.ChatReq) bool { return true }
func (t *Table) OnHosting(*player.Player, bool) bool        { return true }
func (t *Table) OnAutoCallReq(*player.Player, bool) bool    { return true }

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

func (t *Table) OnPlayerActionReq(p *player.Player, in *v1.PlayerActionReq, timeout bool) bool {
	if p == nil || !p.IsGaming() || len(t.GetGamers()) <= 1 || p.GetChairID() != t.active {
		return false
	}
	if s := t.stage.GetState(); s != StPlaying {
		return false
	}

	infoReq := fmt.Sprintf("p=%v, curr=%v, req=%v, pending=%v, Timeout=%v",
		p.Desc(), t.currCard, xgo.ToJSON(in), descPendingEffect(t.pending), timeout)
	log.Debugf("onActionReq. %v", infoReq)

	switch in.Action {
	case v1.ACTION_PLAY_CARD:
		if !t.canOutCard(t.currCard, p.GetCards(), in.OutCard) {
			log.Errorf("playCard err: %v, canOp=%v", infoReq, xgo.ToJSON(t.getCanOp(p)))
			return false
		}
		t.onPlayCard(p, in.OutCard, timeout)

	case v1.ACTION_DRAW_CARD:
		if !t.canDrawCard(p) {
			log.Errorf("drawCard err: 非法摸牌，%v", infoReq)
			return false
		}
		t.onDrawCard(p, timeout)

	case v1.ACTION_SKIP_TURN:
		if t.pending == nil || t.pending.Effect != v1.CARD_EFFECT_SUSPEND {
			log.Errorf("skipTurn err: 当前不允许跳过，%v", infoReq)
			return false
		}
		t.onSkipTurn(p, timeout)

	case v1.ACTION_DECLARE_SUIT:
		if t.pending == nil || t.pending.Effect != v1.CARD_EFFECT_WHOT || !IsWhotCard(t.currCard) {
			log.Errorf("declareSuit err: 当前不允许声明花色, %v", infoReq)
			return false
		}
		if suit := in.DeclareSuit; suit < v1.SUIT_CIRCLE || suit > v1.SUIT_START {
			log.Errorf("declareSuit err: 非法花色, %v", infoReq)
			return false
		}
		t.onDeclareSuit(p, in.DeclareSuit, timeout)

	default:
		log.Warnf("未知操作类型: %v", infoReq)
		return false
	}
	return true
}

func (t *Table) onPlayCard(p *player.Player, card int32, timeout bool) {
	p.RemoveCard(card)
	t.currCard = card
	t.declareSuit = -1
	t.updatePending(p, card)
	t.broadcastPlayerAction(p, v1.ACTION_PLAY_CARD, []int32{card}, 0)
	t.mLog.play(p, card, t.pending, timeout)

	if len(p.GetCards()) == 0 {
		t.updateStage(StWaitEnd)
		return
	}

	// MARKET:14牌：所有其他玩家各抽一张, 发牌不够了游戏结束
	if t.pending != nil && t.pending.Effect == v1.CARD_EFFECT_MARKET && Number(card) == 14 {
		t.pending = nil
		if t.drawCardByMarket(p) {
			t.updateStage(StWaitEnd)
			return
		}
	}

	t.active = t.getNextActiveChair()
	if t.pending != nil {
		t.active = t.pending.Target
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

	pending := &v1.Pending{
		Initiator: p.GetChairID(),
		Target:    p.GetChairID(),
		Effect:    v1.CARD_EFFECT_NORMAL,
		Quantity:  1,
	}

	switch Number(card) {
	case 1:
		pending.Effect = v1.CARD_EFFECT_HOLD_ON
	case 2:
		pending.Effect = v1.CARD_EFFECT_PICK_TWO
		pending.Target = nextChair
		pending.Quantity = 2
	case 8:
		pending.Effect = v1.CARD_EFFECT_SUSPEND
		pending.Target = nextChair
	case 14:
		pending.Effect = v1.CARD_EFFECT_MARKET
	case 20:
		pending.Effect = v1.CARD_EFFECT_WHOT
	}
	t.pending = pending
}

func (t *Table) drawCardByMarket(p *player.Player) (deckEmpty bool) {
	start := p.GetChairID()
	if start < 0 || start >= int32(t.MaxCnt) {
		return false
	}

	chair := start
	for {
		chair = (chair + 1) % int32(t.MaxCnt)
		if chair == start {
			break // 一圈结束，不包括自己
		}

		targetPlayer := t.seats[chair]
		if targetPlayer == nil || !targetPlayer.IsGaming() {
			continue
		}

		drawn := t.cards.DispatchCards(1)
		if len(drawn) == 0 {
			return true // 牌堆空了
		}

		targetPlayer.AddCards(drawn)
		t.sendMarketDrawCardPush(targetPlayer, drawn)
		t.mLog.market(targetPlayer, drawn, t.pending, false)
		log.Debugf("MARKET: uid=%v 抽牌=%v", targetPlayer.Desc(), drawn)
	}

	return t.cards.IsEmpty()
}

func (t *Table) onDrawCard(p *player.Player, timeout bool) {
	count := int32(1)
	if t.pending != nil && t.pending.Quantity > 0 {
		count = t.pending.Quantity
	}

	drawn := t.cards.DispatchCards(int(count))
	if len(drawn) == 0 || t.cards.IsEmpty() {
		log.Debugf("无法摸牌.牌堆不足，结束游戏（剩余=%d）", t.cards.GetCardNum())
		t.updateStage(StWaitEnd)
		return
	}

	if t.pending != nil && t.pending.Target == p.GetChairID() {
		log.Debugf("drawCard. p=%v, 响应了pending，清除: %+v", p.Desc(), descPending(t.pending))
		t.mLog.replyPending(p, v1.ACTION_DRAW_CARD, t.pending)
		t.pending = nil
	}

	p.AddCards(drawn)
	t.broadcastPlayerAction(p, v1.ACTION_DRAW_CARD, drawn, 0)
	t.mLog.draw(p, drawn, t.pending, timeout)

	// 通知下个玩家操作
	t.active = t.getNextActiveChair()
	t.updateStage(StPlaying)
	t.broadcastActivePlayerPush()
}

func (t *Table) onSkipTurn(p *player.Player, timeout bool) {
	t.pending = nil
	t.broadcastPlayerAction(p, v1.ACTION_SKIP_TURN, nil, 0)
	t.mLog.skipTurn(p, timeout)

	// 通知下个玩家操作
	t.active = t.getNextActiveChair()
	t.updateStage(StPlaying)
	t.broadcastActivePlayerPush()
}

func (t *Table) onDeclareSuit(p *player.Player, suit v1.SUIT, timeout bool) {
	t.currCard = NewDeclareWhot(int32(suit), t.currCard) // 修改当前牌的花色
	t.declareSuit = suit
	t.pending = nil
	t.broadcastPlayerAction(p, v1.ACTION_DECLARE_SUIT, nil, suit)
	t.mLog.declareSuit(p, suit, t.currCard, timeout)

	// 通知当前玩家操作
	t.active = p.GetChairID()
	t.updateStage(StPlaying)
	t.broadcastActivePlayerPush()
}

func (t *Table) canOutCard(curr int32, hand []int32, card int32) bool {
	pending := t.pending
	for _, c := range hand {
		if c == card && canPlayCardOn(curr, card, pending) {
			return true
		}
	}
	return false
}

func canPlayCardOn(currCard, card int32, pending *v1.Pending) bool {
	suit, number := Suit(currCard), Number(currCard)
	s, n := Suit(card), Number(card)

	// 没有 pending 情况等价于普通出牌
	if pending == nil {
		return IsWhotCard(card) || s == suit || n == number
	}

	switch pending.Effect {
	case v1.CARD_EFFECT_PICK_TWO, v1.CARD_EFFECT_SUSPEND:
		return !IsWhotCard(card) && n == number

	case v1.CARD_EFFECT_MARKET, v1.CARD_EFFECT_WHOT:
		return false

	default: // CARD_EFFECT_NORMAL, HOLD_ON 等
		return IsWhotCard(card) || s == suit || n == number
	}
}

func (t *Table) canDrawCard(p *player.Player) bool {
	if t.pending == nil {
		return true
	}
	if t.pending.Target != p.GetChairID() {
		log.Errorf("drawCard err: 非目标玩家尝试摸牌: %v", p.Desc())
		return false
	}
	switch t.pending.Effect {
	case v1.CARD_EFFECT_NORMAL, v1.CARD_EFFECT_HOLD_ON, v1.CARD_EFFECT_PICK_TWO:
		return true
	default:
		return false
	}
}
