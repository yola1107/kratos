package table

import (
	"fmt"
	"sort"

	"github.com/yola1107/kratos/v2/log"
	v1 "github.com/yola1107/kratos/v2/ztest/game/whot/api/helloworld/v1"
	"github.com/yola1107/kratos/v2/ztest/game/whot/internal/biz/player"
	"github.com/yola1107/kratos/v2/ztest/game/whot/internal/conf"
	"google.golang.org/protobuf/proto"
)

func (t *Table) SendPacketToClient(p *player.Player, cmd v1.GameCommand, msg proto.Message) {
	if p == nil {
		return
	}
	if p.IsRobot() {
		t.aiLogic.OnMessage(p, cmd, msg)
		return
	}
	session := p.GetSession()
	if session == nil {
		return
	}
	if err := session.Push(int32(cmd), msg); err != nil {
		log.Warnf("send packet to client error: %v", err)
	}
}

func (t *Table) SendPacketToAll(cmd v1.GameCommand, msg proto.Message) {
	for _, v := range t.seats {
		if v == nil {
			continue
		}
		t.SendPacketToClient(v, cmd, msg)
	}
}

func (t *Table) SendPacketToAllExcept(cmd v1.GameCommand, msg proto.Message, uids ...int64) {
	exceptMap := make(map[int64]struct{})
	for _, v := range uids {
		exceptMap[v] = struct{}{}
	}
	for _, v := range t.seats {
		if v == nil {
			continue
		}
		if _, ok := exceptMap[v.GetPlayerID()]; ok {
			continue
		}
		t.SendPacketToClient(v, cmd, msg)
	}
}

// SendLoginRsp 发送玩家登录信息
func (t *Table) SendLoginRsp(p *player.Player, code int32, msg string) {
	t.SendPacketToClient(p, v1.GameCommand_OnLoginRsp, &v1.LoginRsp{
		Code:    code,
		Msg:     msg,
		UserID:  p.GetPlayerID(),
		TableID: p.GetTableID(),
		ChairID: p.GetChairID(),
		ArenaID: int32(conf.ArenaID),
	})
}

// 广播入座信息
func (t *Table) broadcastUserInfo(p *player.Player) {
	t.sendUserInfoToAnother(p, p)
	for k, v := range t.seats {
		if v != nil && k != int(p.GetChairID()) {
			t.sendUserInfoToAnother(p, v)
			t.sendUserInfoToAnother(v, p)
		}
	}
}

func (t *Table) sendUserInfoToAnother(src *player.Player, dst *player.Player) {
	t.SendPacketToClient(dst, v1.GameCommand_OnUserInfoPush, &v1.UserInfoPush{
		UserID:    src.GetPlayerID(),
		ChairID:   src.GetChairID(),
		UserName:  src.GetNickName(),
		Money:     src.GetAllMoney(),
		Avatar:    src.GetAvatar(),
		AvatarUrl: src.GetAvatarUrl(),
		Vip:       src.GetVipGrade(),
		Status:    int32(src.GetStatus()),
		Ip:        src.GetIP(),
	})
}

// BroadcastForwardRsp 消息转发
func (t *Table) BroadcastForwardRsp(ty int32, msg string) {
	t.SendPacketToAll(v1.GameCommand_OnForwardRsp, &v1.ForwardRsp{
		Type: ty,
		Msg:  msg,
	})
}

// 广播玩家断线信息
func (t *Table) broadcastUserOffline(p *player.Player) {
	t.SendPacketToAll(v1.GameCommand_OnUserOfflinePush, &v1.UserOfflinePush{
		UserID:    p.GetPlayerID(),
		IsOffline: p.IsOffline(),
	})
}

// 玩家离桌推送
func (t *Table) broadcastUserQuitPush(p *player.Player, isSwitchTable bool) {
	t.SendPacketToAllExcept(v1.GameCommand_OnPlayerQuitPush, &v1.PlayerQuitPush{
		UserID:  p.GetPlayerID(),
		ChairID: p.GetChairID(),
	}, p.GetPlayerID())
}

// ---------------------------------------------
/*
	游戏协议
*/

func (t *Table) sendMatchOk(p *player.Player) {
	t.SendPacketToClient(p, v1.GameCommand_OnMatchResultPush, &v1.MatchResultPush{
		Code: 0,
		Msg:  "",
		Uid:  p.GetPlayerID(),
	})
}

// 发牌推送
func (t *Table) dispatchCardPush(canGameSeats []*player.Player, bottom []int32, leftNum int32) {
	for _, p := range canGameSeats {
		if p == nil {
			continue
		}
		if !p.IsGaming() {
			continue
		}
		t.SendPacketToClient(p, v1.GameCommand_OnSendCardPush, &v1.SendCardPush{
			UserID:  p.GetPlayerID(),
			Cards:   p.GetCards(),
			Bottom:  bottom,
			LeftNum: leftNum,
		})
	}
}

// SendSceneInfo 发送游戏场景信息
func (t *Table) SendSceneInfo(p *player.Player) {
	c := t.repo.GetRoomConfig()
	rsp := &v1.SceneRsp{
		BaseScore:   c.Game.BaseMoney,
		Stage:       int32(t.stage.GetState()),
		Timeout:     int64(t.stage.Remaining().Seconds()),
		Active:      t.active,
		FirstChair:  t.first,
		CurrCard:    t.currCard,
		DeclareSuit: t.declareSuit,
		LeftNum:     t.cards.GetCardNum(),
		Pending:     t.pending,
		Players:     t.getPlayersScene(),
	}
	t.SendPacketToClient(p, v1.GameCommand_OnSceneRsp, rsp)
}

func (t *Table) getPlayersScene() (players []*v1.PlayerInfo) {
	for _, p := range t.seats {
		if p == nil {
			continue
		}
		players = append(players, t.getScene(p))
	}
	return
}

func (t *Table) getScene(p *player.Player) *v1.PlayerInfo {
	if p == nil {
		return nil
	}
	info := &v1.PlayerInfo{
		UserId:  p.GetPlayerID(),
		ChairId: p.GetChairID(),
		Status:  int32(p.GetStatus()),
		Hosting: p.GetTimeoutCnt() > 0,
		Offline: p.IsOffline(),
		Cards:   p.GetCards(),
		CanOp:   t.getCanOp(p),
	}
	return info
}

// 当前活动玩家推送
func (t *Table) broadcastActivePlayerPush() {
	for _, p := range t.seats {
		if p == nil {
			continue
		}
		rsp := &v1.ActivePush{
			Stage:   int32(t.stage.GetState()),
			Timeout: int64(t.stage.Remaining().Seconds()),
			Active:  t.active,
			LeftNum: t.cards.GetCardNum(),
			Pending: t.pending,
			CanOp:   t.getCanOp(p),
		}
		if p.GetChairID() == t.active && p.IsGaming() {
			t.mLog.activePush(p, t.currCard, t.pending, rsp.CanOp)
			// log.Debugf("sendActivePush. p:%v, curr=%d, pending=%s, canOp=%v,",
			// 	p.Desc(), t.currCard, descPendingEffect(t.pending), ext.ToJSON(rsp.CanOp))
		}
		t.SendPacketToClient(p, v1.GameCommand_OnActivePush, rsp)
	}
}

func (t *Table) broadcastPlayerAction(p *player.Player, action v1.ACTION, cs []int32, declaredSuit v1.SUIT) {
	for _, v := range t.seats {
		if v == nil {
			continue
		}

		hand := p.GetCards()
		self := v.GetPlayerID() == p.GetPlayerID()

		rsp := &v1.PlayerActionRsp{
			Code:    0,
			Message: "",
			UserId:  p.GetPlayerID(),
			ChairId: p.GetChairID(),
			Action:  action,
			LeftNum: t.cards.GetCardNum(),
			Effect:  t.pending,
			PlayResult: &v1.PlayCardResult{
				Card:       0,
				Cards:      nil,
				TotalCards: int32(len(hand)),
			},
			DrawResult: &v1.DrawCardResult{
				DrawNum:    0,
				Cards:      nil,
				TotalCards: int32(len(hand)),
			},
			DeclareResult: &v1.DeclareSuitResult{
				Suit: declaredSuit,
			},
		}

		switch action {
		case v1.ACTION_PLAY_CARD:
			if len(cs) == 1 {
				rsp.PlayResult.Card = cs[0]
			}
			if self {
				rsp.PlayResult.Cards = hand
			}
		case v1.ACTION_DRAW_CARD:
			rsp.DrawResult.DrawNum = int32(len(cs))
			if self {
				rsp.DrawResult.Drawn = cs
				rsp.PlayResult.Cards = hand
			}
		}

		t.SendPacketToClient(v, v1.GameCommand_OnPlayerActionRsp, rsp)
	}

	deSuit := ""
	if declaredSuit != 0 {
		deSuit = fmt.Sprintf("%d", declaredSuit)
	}
	pendingStr := descPendingEffect(t.pending)
	log.Debugf("sendActionRsp. p=%v, ac=%q, cards=%v, deSuit=%v, pending=%v,curr=%v, \n",
		p.Desc(), action, cs, deSuit, pendingStr, t.currCard)
}

func (t *Table) getCanOp(p *player.Player) []*v1.ActionOption {
	if p == nil || !p.IsGaming() || len(t.GetGamers()) <= 1 || p.GetChairID() != t.active {
		return nil
	}
	switch t.stage.GetState() {
	case StWait, StReady, StWaitEnd, StEnd:
		return nil
	default:
	}

	var ops []*v1.ActionOption
	pending := t.pending

	switch {
	case pending == nil || pending.Effect == v1.CARD_EFFECT_NORMAL, pending.Effect == v1.CARD_EFFECT_HOLD_ON:
		ops = append(ops, newDrawOption(1))

	case pending.Effect == v1.CARD_EFFECT_PICK_TWO: // 出牌出点数一样的
		ops = append(ops, newDrawOption(pending.Quantity))

	case pending.Effect == v1.CARD_EFFECT_SUSPEND: // 出牌出点数一样的
		ops = append(ops, &v1.ActionOption{Action: v1.ACTION_SKIP_TURN})

	case pending.Effect == v1.CARD_EFFECT_WHOT:
		suits := []v1.SUIT(nil)
		for i := v1.SUIT_CIRCLE; i <= v1.SUIT_START; i++ {
			suits = append(suits, i)
		}
		return []*v1.ActionOption{
			{Action: v1.ACTION_DECLARE_SUIT, Suits: suits},
		}
	}

	canOuts := collectPlayCard(t.currCard, p.GetCards(), pending)
	if len(canOuts) > 0 {
		ops = append(ops, &v1.ActionOption{Action: v1.ACTION_PLAY_CARD, Cards: canOuts})
	}

	if len(ops) > 1 {
		sort.Slice(ops, func(i, j int) bool {
			return ops[i].Action < ops[j].Action
		})
	}

	return ops
}

func newDrawOption(n int32) *v1.ActionOption {
	return &v1.ActionOption{Action: v1.ACTION_DRAW_CARD, DrawCount: n}
}

func collectPlayCard(currCard int32, hand []int32, pending *v1.Pending) []int32 {
	out := make([]int32, 0, len(hand))
	for _, card := range hand {
		if canPlayCardOn(currCard, card, pending) {
			out = append(out, card)
		}
	}
	return out
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
		return IsWhotCard(card) || n == number

	case v1.CARD_EFFECT_MARKET, v1.CARD_EFFECT_WHOT:
		return false

	default: // CARD_EFFECT_NORMAL, HOLD_ON 等
		return IsWhotCard(card) || s == suit || n == number
	}
}

func (t *Table) sendMarketDrawCardPush(p *player.Player, draw []int32) {
	t.SendPacketToClient(p, v1.GameCommand_OnMarketDrawCardPush, &v1.MarketDrawCardPush{
		UserID:  p.GetPlayerID(),
		ChairID: p.GetChairID(),
		Draw:    draw,
		Cards:   p.GetCards(),
		LeftNum: t.cards.GetCardNum(),
	})
}

func (t *Table) broadcastResult(obj *SettleObj) {
	rsp := &v1.ResultPush{}
	if obj != nil {
		rsp = obj.GetResult()
	}
	t.SendPacketToAll(v1.GameCommand_OnResultPush, rsp)
}

func ifThen[T any](cond bool, a, b T) T {
	if cond {
		return a
	}
	return b
}

func ifThenInt32(cond bool, a, b int32) int32 {
	if cond {
		return a
	}
	return b
}
