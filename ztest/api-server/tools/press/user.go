package press

import (
	"context"
	"sync/atomic"
	"time"

	gproto "github.com/golang/protobuf/proto"
	"github.com/yola1107/kratos/v2/library/ext"
	"github.com/yola1107/kratos/v2/library/work"
	"github.com/yola1107/kratos/v2/log"
	"github.com/yola1107/kratos/v2/transport/websocket"
	v1 "github.com/yola1107/kratos/v2/ztest/api-server/api/helloworld/v1"
	"github.com/yola1107/kratos/v2/ztest/api-server/internal/biz/table"
)

type Repo interface {
	GetTimer() work.ITaskScheduler
	GetLoop() work.ITaskLoop
	GetContext() context.Context
	GetUrl() string
}

type User struct {
	repo     Repo
	id       int64
	login    atomic.Bool
	chair    atomic.Int32
	activeAt atomic.Int64
	client   atomic.Pointer[websocket.Client] // *websocket.Client
}

func NewUser(id int64, repo Repo) (*User, error) {
	u := &User{
		repo: repo,
		id:   id,
	}
	u.chair.Store(-1)
	repo.GetLoop().Post(u.Init)
	return u, nil
}

func (u *User) Reset() {
	// u.login.Store(false)
	u.chair.Store(-1)
	u.activeAt.Store(0)
}

func (u *User) IsFree() bool {
	return time.Now().Unix()-u.activeAt.Load() >= 60 // 30s
}

func (u *User) UpActiveAt() {
	u.activeAt.Store(time.Now().Unix())
}

func (u *User) Release() {
	client := u.client.Load()
	if client == nil {
		return
	}
	client.Close()
	client = nil
}

func (u *User) Init() {
	pushHandler := map[int32]websocket.PushHandler{
		int32(v1.GameCommand_SayHelloRsp):          u.OnEmptyPush,
		int32(v1.GameCommand_OnLoginRsp):           u.OnLoginRsp,   // GameCommand = 1002
		int32(v1.GameCommand_OnLogoutRsp):          u.OnEmptyPush,  // GameCommand = 1004
		int32(v1.GameCommand_OnReadyRsp):           u.OnEmptyPush,  // GameCommand = 1006
		int32(v1.GameCommand_OnSwitchTableRsp):     u.OnEmptyPush,  // GameCommand = 1008
		int32(v1.GameCommand_OnSceneRsp):           u.OnEmptyPush,  // GameCommand = 1010
		int32(v1.GameCommand_OnChatRsp):            u.OnEmptyPush,  // GameCommand = 1012
		int32(v1.GameCommand_OnHostingRsp):         u.OnEmptyPush,  // GameCommand = 1014
		int32(v1.GameCommand_OnForwardRsp):         u.OnEmptyPush,  // GameCommand = 1016
		int32(v1.GameCommand_OnActionRsp):          u.OnEmptyPush,  // GameCommand = 1102
		int32(v1.GameCommand_OnAutoCallRsp):        u.OnEmptyPush,  // GameCommand = 1104
		int32(v1.GameCommand_OnUserInfoPush):       u.OnEmptyPush,  // GameCommand = 2001
		int32(v1.GameCommand_OnEmojiConfigPush):    u.OnEmptyPush,  // GameCommand = 2002
		int32(v1.GameCommand_OnPlayerQuitPush):     u.OnEmptyPush,  // GameCommand = 2003
		int32(v1.GameCommand_OnUserOfflinePush):    u.OnEmptyPush,  // GameCommand = 2004
		int32(v1.GameCommand_OnSetBankerPush):      u.OnEmptyPush,  // GameCommand = 2100
		int32(v1.GameCommand_OnSendCardPush):       u.OnEmptyPush,  // GameCommand = 2101
		int32(v1.GameCommand_OnActivePush):         u.OnActivePush, // GameCommand = 2102 //活动玩家通知
		int32(v1.GameCommand_OnRoundSeePush):       u.OnEmptyPush,  // GameCommand = 2103
		int32(v1.GameCommand_OnAfterSeeButtonPush): u.OnEmptyPush,  // GameCommand = 2104
		int32(v1.GameCommand_OnShowCardPush):       u.OnEmptyPush,  // GameCommand = 2105
		int32(v1.GameCommand_OnResultPush):         u.OnResultPush, // GameCommand = 2200 //结算通知
	}
	rspHandler := map[int32]websocket.ResponseHandler{
		int32(v1.GameCommand_SayHelloReq):      u.OnEmptyRequest, // 空
		int32(v1.GameCommand_OnLoginReq):       u.OnEmptyRequest, // GameCommand = 1001 //登录
		int32(v1.GameCommand_OnLogoutReq):      u.OnEmptyRequest, // GameCommand = 1003 //登出
		int32(v1.GameCommand_OnReadyReq):       u.OnEmptyRequest, // GameCommand = 1005 //准备
		int32(v1.GameCommand_OnSwitchTableReq): u.OnEmptyRequest, // GameCommand = 1007 //换桌
		int32(v1.GameCommand_OnSceneReq):       u.OnEmptyRequest, // GameCommand = 1009 //场景信息
		int32(v1.GameCommand_OnChatReq):        u.OnEmptyRequest, // GameCommand = 1011 //聊天
		int32(v1.GameCommand_OnHostingReq):     u.OnEmptyRequest, // GameCommand = 1013 //托管
		int32(v1.GameCommand_OnForwardReq):     u.OnEmptyRequest, // GameCommand = 1015 //转发
		int32(v1.GameCommand_OnActionReq):      u.OnEmptyRequest, // GameCommand = 1101 //玩家动作
		int32(v1.GameCommand_OnAutoCallReq):    u.OnEmptyRequest, // GameCommand = 1103 //自动跟注
	}
	wsClient, err := websocket.NewClient(
		u.repo.GetContext(),
		websocket.WithEndpoint(u.repo.GetUrl()),
		websocket.WithToken(""),
		websocket.WithPushHandler(pushHandler),
		websocket.WithResponseHandler(rspHandler),
		websocket.WithConnectFunc(u.OnConnect),
		websocket.WithDisconnectFunc(u.OnDisconnect),
	)
	if err != nil {
		log.Errorf("err=%q", err)
		return
	}
	u.client.Store(wsClient)

	// login
	dur := time.Duration(ext.RandInt(0, 10000)) * time.Millisecond
	u.repo.GetTimer().Once(dur, func() {
		u.Request(v1.GameCommand_OnLoginReq, &v1.LoginReq{
			UserID: u.id,
			Token:  "token",
		})
	})
}

func (u *User) OnEmptyPush(data []byte)                {}
func (u *User) OnEmptyRequest(data []byte, code int32) {}

func (u *User) OnConnect(session *websocket.Session) {
	log.Debugf("connect called. uid=%d %q ", u.id, session.ID())
}

func (u *User) OnDisconnect(session *websocket.Session) {
	log.Debugf("disconnect called. uid=%d %q ", u.id, session.ID())
	u.Reset()
}

func (u *User) Request(cmd v1.GameCommand, msg gproto.Message) {
	wsClient := u.client.Load()
	if wsClient == nil {
		log.Warnf("wsClient is nil")
		return
	}
	if !wsClient.IsAlive() {
		return
	}
	if err := wsClient.Request(int32(cmd), msg); err != nil {
		log.Errorf("%v", err)
		return
	}
	u.UpActiveAt()
}

func (u *User) OnLoginRsp(data []byte) {
	rsp := &v1.LoginRsp{}
	if err := gproto.Unmarshal(data, rsp); err != nil {
		log.Errorf("%v", err)
		return
	}
	if rsp.Code != 0 {
		log.Errorf("loginRsp. uid=%d code=%d msg=%q", u.id, rsp.Code, rsp.Msg)
		return
	}
	u.login.Store(true)
	u.chair.Store(rsp.ChairID)
	u.UpActiveAt()
}

func (u *User) OnActivePush(data []byte) {
	rsp := &v1.ActivePush{}
	if err := gproto.Unmarshal(data, rsp); err != nil {
		log.Errorf("%v", err)
		return
	}
	if u.chair.Load() != rsp.Active {
		return
	}
	op := table.RandOpWithWeight(rsp.CanOp)
	req := &v1.ActionReq{
		UserID:         u.id,
		Action:         op,
		SideReplyAllow: ext.IsHitFloat(0.3),
	}
	dur := time.Duration(ext.RandInt(0, 12000)) * time.Millisecond
	u.repo.GetTimer().Once(dur, func() {
		u.Request(v1.GameCommand_OnActionReq, req)
	})
}

func (u *User) OnResultPush(data []byte) {
	rsp := &v1.ResultPush{}
	if err := gproto.Unmarshal(data, rsp); err != nil {
		log.Errorf("%v", err)
		return
	}
	if rsp.UserID != u.id {
		return
	}
	u.chair.Store(-1)
	if ext.IsHitFloat(0.5) {
		return
	}
	req := &v1.LogoutReq{
		UserDBID: u.id,
	}
	dur := time.Duration(ext.RandInt(0, 5000)) * time.Millisecond
	u.repo.GetTimer().Once(dur, func() {
		u.Request(v1.GameCommand_OnLogoutRsp, req)
	})
}
