package table

import (
	"github.com/yola1107/kratos/v2/library/ext"
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
}

func (t *Table) OnReadyReq(p *player.Player, isReady bool) bool     { return true }
func (t *Table) OnChatReq(p *player.Player, in *v1.ChatReq) bool    { return true }
func (t *Table) OnHosting(p *player.Player, isHosting bool) bool    { return true }
func (t *Table) OnAutoCallReq(p *player.Player, autoCall bool) bool { return true }

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

func (t *Table) OnPlayerActionReq(p *player.Player, in *v1.PlayerActionReq, timeout bool) (ok bool) {
	if p == nil || !p.IsGaming() || len(t.GetGamers()) <= 1 || p.GetChairID() != t.active {
		return
	}

	s := t.stage.GetState()
	if s == StWait || s == StReady || s == StWaitEnd || s == StEnd {
		return
	}

	reqstr := ext.ToJSON(in)
	pendstr := descPendingEffect(t.pending)
	log.Debugf("onActionReq. p=%v, req=%v, pending=%v, Timeout=%v ",
		p.Desc(), reqstr, pendstr, timeout)

	switch in.Action {
	case v1.ACTION_PLAY_CARD:
		if !t.canOutCard(t.currCard, p.GetCards(), in.OutCard) {
			log.Errorf("playCard err: p=%v, curr=%v, out=%v,", p.Desc(), t.currCard, reqstr)
			return
		}
		t.onPlayCard(p, in.OutCard, timeout)

	case v1.ACTION_DRAW_CARD:
		if !t.canDrawCard(p) {
			log.Errorf("drawCard err: 当前不允许摸牌，p=%v, currCard=%d, %v", p.Desc(), t.currCard, pendstr)
			return
		}
		t.onDrawCard(p, timeout)

	case v1.ACTION_SKIP_TURN: // 8牌
		if t.pending == nil || t.pending.Effect != v1.CARD_EFFECT_SUSPEND {
			log.Errorf("skipCard err: 当前不允许跳过，p=%v, currCard=%d %v", p.Desc(), t.currCard, pendstr)
			return
		}
		t.onSkipTurn(p, timeout)

	case v1.ACTION_DECLARE_SUIT:
		if t.pending == nil || t.pending.Effect != v1.CARD_EFFECT_WHOT || t.currCard != WhotCard {
			log.Errorf("declareCard err: 当前不允许声明花色, p=%v, currCard=%d %v ", p.Desc(), t.currCard, pendstr)
			return
		}
		if suit := in.DeclareSuit; suit < v1.SUIT_CIRCLE || suit > v1.SUIT_START {
			log.Errorf("declareCard err: 非法花色, p=%v, suit=%d", p.Desc(), in.DeclareSuit)
			return
		}
		t.onDeclareSuit(p, in.DeclareSuit, timeout)

	default:
		log.Warnf("未知操作类型: %v", in.Action)
		return
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

	// 14牌：所有其他玩家各抽一张 MARKET, 发牌不够了游戏结束
	if t.pending != nil && t.pending.Effect == v1.CARD_EFFECT_MARKET && Number(card) == 14 {
		t.pending = nil // 清理掉14牌等待响应
		if over := t.drawCardByMarket(p); over {
			t.updateStage(StWaitEnd)
			return
		}
	}

	next := t.getNextActiveChair()
	if t.pending != nil {
		next = t.pending.Target
	}
	t.active = next
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

	t.pending = &v1.Pending{
		Initiator: p.GetChairID(),
		Target:    p.GetChairID(),
		Effect:    v1.CARD_EFFECT_NORMAL,
		Quantity:  1,
	}

	switch Number(card) {
	case 1: // Hold On
		t.pending.Effect = v1.CARD_EFFECT_HOLD_ON
	case 2: // Pick Two
		t.pending.Effect = v1.CARD_EFFECT_PICK_TWO
		t.pending.Target = nextChair
		t.pending.Quantity = 2
	case 8: // Suspend
		t.pending.Effect = v1.CARD_EFFECT_SUSPEND
		t.pending.Target = nextChair
	case 14: // Market
		t.pending.Effect = v1.CARD_EFFECT_MARKET
	case 20: // WHOT（需要声明花色）
		t.pending.Effect = v1.CARD_EFFECT_WHOT
	}
}

func (t *Table) drawCardByMarket(p *player.Player) (end bool) {
	for _, v := range t.seats {
		if v == nil || !v.IsGaming() {
			continue
		}
		if v.GetPlayerID() == p.GetPlayerID() {
			continue
		}
		drawn := t.cards.DispatchCards(1)
		if len(drawn) == 0 || t.cards.IsEmpty() {
			return true
		}
		v.AddCards(drawn)
		t.sendMarketDrawCardPush(v, drawn)
		t.mLog.market(v, drawn, t.pending, false)
		log.Debugf("drawCardByMarket: 所有其他玩家各抽一张. uid=%v, drawn=%v ", v.GetPlayerID(), drawn)
	}
	return false
}

func (t *Table) onDrawCard(p *player.Player, timeout bool) {
	count := int32(1)
	if t.pending != nil && t.pending.Quantity > 0 {
		count = t.pending.Quantity
	}

	drawn := t.cards.DispatchCards(int(count))
	if len(drawn) == 0 || t.cards.IsEmpty() {
		log.Debugf("无法摸牌（可能牌堆不足 leftNum=%d），结束游戏", t.cards.GetCardNum())
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
	t.currCard = NewDeclareWhot(int32(suit)) // 修改当前牌的花色
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

func (t *Table) canDrawCard(p *player.Player) bool {
	if t.pending == nil {
		return true
	}

	if t.pending != nil && t.pending.Target != p.GetChairID() {
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
