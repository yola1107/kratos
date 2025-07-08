package player

import (
	"github.com/yola1107/kratos/v2/log"
	"github.com/yola1107/kratos/v2/transport/websocket"
	v1 "github.com/yola1107/kratos/v2/ztest/api-server/api/helloworld/v1"
	"github.com/yola1107/kratos/v2/ztest/api-server/internal/conf"
	"google.golang.org/protobuf/proto"
)

type Player struct {
	isRobot  bool
	session  *websocket.Session
	gameData *GameData
	baseData *BaseData // 私有，不暴露
}

type Raw struct {
	ID       int64
	IsRobot  bool
	Session  *websocket.Session
	BaseData *BaseData
}

func New(raw *Raw) *Player {
	p := &Player{
		isRobot:  raw.IsRobot,
		session:  raw.Session,
		gameData: &GameData{},
		baseData: raw.BaseData,
	}
	return p
}

func (p *Player) SetBaseData(baseData *BaseData) {
	p.baseData = baseData
}

func (p *Player) GetBaseData() *BaseData {
	return p.baseData
}

func (p *Player) IsRobot() bool {
	return p.isRobot
}

func (p *Player) GetSessionID() string {
	if p.session == nil {
		return ""
	}
	return p.session.ID()
}

func (p *Player) GetSession() *websocket.Session {
	return p.session
}

func (p *Player) UpdateSession(session *websocket.Session) {
	p.session = session
}

func (p *Player) GetIP() string {
	if p.session == nil {
		return ""
	}
	return p.session.GetRemoteIP()
}

func (p *Player) LogoutGame(code int32, msg string) {
	// 通知客户端退出
	p.SendLogout(code, msg)

	if session := p.GetSession(); session != nil {
		session.Close(true)
		session = nil
	}

	// clean
	p.session = nil
	p.gameData = nil
	p.baseData = nil
}

func (p *Player) push(cmd v1.GameCommand, msg proto.Message) {
	if p == nil {
		return
	}
	if p.IsRobot() {
		return
	}
	if p.session == nil {
		return
	}
	if err := p.session.Push(int32(cmd), msg); err != nil {
		log.Warnf("send packet to client error: %v", err)
	}
}

func (p *Player) SendLoginRsp(code int32, msg string) {
	if p == nil {
		return
	}
	p.push(v1.GameCommand_OnLoginRsp, &v1.LoginRsp{
		Code:    code,
		Msg:     msg,
		UserID:  p.GetPlayerID(),
		TableID: p.GetTableID(),
		ChairID: p.GetChairID(),
		ArenaID: int32(conf.ArenaID),
	})
}

func (p *Player) SendSwitchTableRsp(code int32, msg string) {
	if p == nil {
		return
	}
	p.push(v1.GameCommand_OnSwitchTableRsp, &v1.SwitchTableRsp{
		Code:   code,
		Msg:    msg,
		UserID: p.GetPlayerID(),
	})
}

func (p *Player) SendLogout(code int32, msg string) {
	if p == nil {
		return
	}
	p.push(v1.GameCommand_OnLogoutRsp, &v1.LogoutRsp{
		Code:   code,
		Msg:    msg,
		UserID: p.GetPlayerID(),
	})
}
