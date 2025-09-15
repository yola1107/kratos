package table

import (
	"github.com/yola1107/kratos/v2/library/xgo"
	"github.com/yola1107/kratos/v2/log"
	v1 "github.com/yola1107/kratos/v2/ztest/game/ludo/api/helloworld/v1"
	"github.com/yola1107/kratos/v2/ztest/game/ludo/internal/biz/player"
	"github.com/yola1107/kratos/v2/ztest/game/ludo/internal/conf"
	"github.com/yola1107/kratos/v2/ztest/game/ludo/internal/model"
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
func (t *Table) dispatchCardPush(canGameSeats []*player.Player) {
	pieces := t.GetBoardAllPieces()
	for _, p := range canGameSeats {
		if p == nil || !p.IsGaming() {
			continue
		}
		t.SendPacketToClient(p, v1.GameCommand_OnSendCardPush, &v1.SendCardPush{
			UserID:     p.GetPlayerID(),
			FirstChair: t.first,
			Color:      p.GetColor(),
			Pieces:     pieces,
		})
	}
}

func (t *Table) GetBoardAllPieces() []*v1.Piece {
	if t.board == nil {
		return nil
	}
	pieces := []*v1.Piece(nil)
	for _, piece := range t.board.Pieces() {
		pieces = append(pieces, &v1.Piece{
			Id:     piece.ID(),
			Pos:    piece.Pos(),
			Color:  piece.Color(),
			Status: piece.Status(),
		})
	}
	return pieces
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
		BoardConfig: t.getBoardConfig(),
		Players:     t.getPlayersScene(),
		Pieces:      t.GetBoardAllPieces(),
	}
	t.SendPacketToClient(p, v1.GameCommand_OnSceneRsp, rsp)
}

func (t *Table) getBoardConfig() *v1.BoardConfig {
	common := []int32(nil)
	for i := 0; i < model.TotalPositions; i++ {
		common = append(common, int32(i))
	}
	safe := []int32(nil)
	for k, _ := range model.SafePositions {
		safe = append(safe, k)
	}
	b := &v1.BoardConfig{
		Common: common,
		Home:   []int32{-1, -1, -1, -1},
		Entry:  model.EntryPoints,
		Safe:   safe,
		End:    model.HomeStartIndices,
		Color:  []int32{0, 1, 2, 3},
	}
	return b
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
		UserId:    p.GetPlayerID(),
		ChairId:   p.GetChairID(),
		Status:    int32(p.GetStatus()),
		Hosting:   p.GetTimeoutCnt() > 0,
		Offline:   p.IsOffline(),
		Color:     p.GetColor(),
		DiceList:  t.getDiceList(p),
		CanAction: 0,
		MoveDices: nil,
		Ret:       nil,
	}
	if p.IsGaming() && !p.IsFinish() {
		info.CanAction, info.MoveDices, info.Ret = t.getCanAction(p)
	}
	return info
}

func (t *Table) getDiceList(p *player.Player) []*v1.Dice {
	list := p.GetDiceSlot()
	if p.GetChairID() != t.active {
		list = p.GetLastDiceSlot()
	}

	ret := []*v1.Dice(nil)
	for _, v := range list {
		ret = append(ret, &v1.Dice{
			Value: v.Value,
			Used:  v.Used,
		})
	}
	return ret
}

// 当前活动玩家推送
func (t *Table) broadcastActivePlayerPush() {
	for _, p := range t.seats {
		if p == nil {
			continue
		}
		rsp := &v1.ActivePush{
			Stage:       int32(t.stage.GetState()),
			Timeout:     int64(t.stage.Remaining().Seconds()),
			Active:      t.active,
			UnusedDices: p.UnusedDice(), // 当前玩家的色子列表
			CanAction:   0,
			MoveDices:   nil,
			Ret:         nil,
		}
		if p.GetChairID() == t.active && p.IsGaming() && !p.IsFinish() {
			rsp.CanAction, rsp.MoveDices, rsp.Ret = t.getCanAction(p)
			// log debug.
			canMoveDiceStr, retStr := "", ""
			if rsp.CanAction == v1.ACTION_TYPE_AcMove {
				canMoveDiceStr, retStr = xgo.ToJSON(rsp.MoveDices), p.DescPath()
			}
			t.mLog.activePush(p, rsp.CanAction, canMoveDiceStr, retStr)
			log.Debugf("ActivePush. p:%v, canOp=%q, moveDices=%v, ret=%v\n",
				p.Desc(), rsp.CanAction, canMoveDiceStr, retStr)
		}
		t.SendPacketToClient(p, v1.GameCommand_OnActivePush, rsp)
	}
}

func (t *Table) getCanAction(p *player.Player) (v1.ACTION_TYPE, []*v1.CanMoveDice, *v1.TagRetData) {
	if p == nil || !p.IsGaming() || p.GetChairID() != t.active {
		return 0, nil, nil
	}
	switch t.stage.GetState() {
	case StDice:
		return v1.ACTION_TYPE_AcDice, nil, nil
	case StMove:
		return v1.ACTION_TYPE_AcMove, t.getCanMoveDice(p), t.getTagRetData(p)
	default:
		return 0, nil, nil
	}
}

func (t *Table) getCanMoveDice(p *player.Player) []*v1.CanMoveDice {
	res := make([]*v1.CanMoveDice, 0)
	moves := t.board.CalcCanMoveDice(p.GetColor(), p.UnusedDice())
	for _, m := range moves {
		res = append(res, &v1.CanMoveDice{
			Dice:   m.Dice,
			Pieces: m.Pieces,
		})
	}
	return res
}

func (t *Table) getTagRetData(p *player.Player) *v1.TagRetData {
	r := p.GetPaths()
	if r == nil {
		log.Errorf("getTagRetData. paths is nil")
		return nil
	}
	paths := make([]*v1.Path, 0)
	for _, v := range r.Dst {
		paths = append(paths, &v1.Path{
			Path: v,
		})
	}
	return &v1.TagRetData{
		Max:   int32(r.Max),
		Cache: r.Cache,
		Paths: paths,
	}
}

func (t *Table) broadcastDiceRsp(p *player.Player, dice int32) {
	diceList := []*v1.Dice(nil)
	for _, v := range p.GetDiceSlot() {
		diceList = append(diceList, &v1.Dice{
			Value: v.Value,
			Used:  v.Used,
		})
	}
	t.SendPacketToAll(v1.GameCommand_OnDiceRsp, &v1.DiceRsp{
		Code:     0,
		Msg:      "",
		Uid:      p.GetPlayerID(),
		Dice:     dice,
		DiceList: diceList,
	})
}

func (t *Table) broadcastMoveRsp(p *player.Player, pieceId, dice int32, delta map[int32]int64, step *model.Step) {
	move := &v1.DiceMove{}
	Killed := []*v1.DiceMove(nil)

	if step != nil {
		move = &v1.DiceMove{
			PlayerId: p.GetPlayerID(),
			PieceId:  step.Id,
			From:     step.From,
			To:       step.To,
		}
		for _, kill := range step.Killed {
			Killed = append(Killed, &v1.DiceMove{
				PlayerId: t.colorMap[kill.Color],
				PieceId:  kill.Id,
				From:     kill.From,
				To:       kill.To,
			})
		}
	}

	t.SendPacketToAll(v1.GameCommand_OnMoveRsp, &v1.MoveRsp{
		Code:      0,
		Msg:       "",
		DiceValue: dice,
		Move:      move,
		Killed:    Killed,
		Pieces:    t.GetBoardAllPieces(),
	})
}

func (t *Table) broadcastResult(obj *SettleObj) {
	rsp := &v1.ResultPush{}
	if obj != nil {
		rsp = obj.GetResult()
	}
	t.SendPacketToAll(v1.GameCommand_OnResultPush, rsp)
}
