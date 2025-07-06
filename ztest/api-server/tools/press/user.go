package press

import (
	"context"
	"sync/atomic"
	"time"

	gproto "github.com/golang/protobuf/proto"
	"github.com/yola1107/kratos/v2/library/work"
	"github.com/yola1107/kratos/v2/log"
	"github.com/yola1107/kratos/v2/transport/websocket"
	v1 "github.com/yola1107/kratos/v2/ztest/api-server/api/helloworld/v1"
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
	activeAt atomic.Int64
	client   atomic.Pointer[websocket.Client] // *websocket.Client
}

func NewUser(id int64, repo Repo) (*User, error) {
	u := &User{
		repo: repo,
		id:   id,
	}
	repo.GetLoop().Post(u.Login)
	return u, nil
}

func (u *User) IsFree() bool {
	return time.Now().Unix()-u.activeAt.Load() >= 30 // 30s
}

func (u *User) SetActiveAt() {
	u.activeAt.Store(time.Now().Unix())
}

func (u *User) Release() {
	if u.client.Load() == nil {
		return
	}
	u.Request(v1.GameCommand_OnLogoutReq, &v1.LogoutReq{
		UserDBID: u.id,
	})
}

func (u *User) Login() {
	pushHandler := map[int32]websocket.PushHandler{
		int32(v1.GameCommand_SayHelloRsp):          u.OnMsgPush,
		int32(v1.GameCommand_OnLoginRsp):           u.OnMsgPush, // GameCommand = 1002
		int32(v1.GameCommand_OnLogoutRsp):          u.OnMsgPush, // GameCommand = 1004
		int32(v1.GameCommand_OnReadyRsp):           u.OnMsgPush, // GameCommand = 1006
		int32(v1.GameCommand_OnSwitchTableRsp):     u.OnMsgPush, // GameCommand = 1008
		int32(v1.GameCommand_OnSceneRsp):           u.OnMsgPush, // GameCommand = 1010
		int32(v1.GameCommand_OnChatRsp):            u.OnMsgPush, // GameCommand = 1012
		int32(v1.GameCommand_OnHostingRsp):         u.OnMsgPush, // GameCommand = 1014
		int32(v1.GameCommand_OnForwardRsp):         u.OnMsgPush, // GameCommand = 1016
		int32(v1.GameCommand_OnActionRsp):          u.OnMsgPush, // GameCommand = 1102
		int32(v1.GameCommand_OnAutoCallRsp):        u.OnMsgPush, // GameCommand = 1104
		int32(v1.GameCommand_OnUserInfoPush):       u.OnMsgPush, // GameCommand = 2001 //玩家信息
		int32(v1.GameCommand_OnEmojiConfigPush):    u.OnMsgPush, // GameCommand = 2002 //表情道具配置
		int32(v1.GameCommand_OnPlayerQuitPush):     u.OnMsgPush, // GameCommand = 2003 //玩家退出
		int32(v1.GameCommand_OnUserOfflinePush):    u.OnMsgPush, // GameCommand = 2004 //用户断线通知
		int32(v1.GameCommand_OnSetBankerPush):      u.OnMsgPush, // GameCommand = 2100 //庄家通知
		int32(v1.GameCommand_OnSendCardPush):       u.OnMsgPush, // GameCommand = 2101 //发牌通知
		int32(v1.GameCommand_OnActivePush):         u.OnMsgPush, // GameCommand = 2102 //活动玩家通知
		int32(v1.GameCommand_OnRoundSeePush):       u.OnMsgPush, // GameCommand = 2103 //自动看牌通知
		int32(v1.GameCommand_OnAfterSeeButtonPush): u.OnMsgPush, // GameCommand = 2104 //按钮通知(看牌后触发新增按钮)
		int32(v1.GameCommand_OnShowCardPush):       u.OnMsgPush, // GameCommand = 2105 //亮牌通知
		int32(v1.GameCommand_OnResultPush):         u.OnMsgPush, // GameCommand = 2200 //结算通知
	}
	rspHandler := map[int32]websocket.ResponseHandler{
		int32(v1.GameCommand_SayHelloReq):      u.OnMsgRsp, // 空
		int32(v1.GameCommand_OnLoginReq):       u.OnMsgRsp, // GameCommand = 1001 //登录
		int32(v1.GameCommand_OnLogoutReq):      u.OnMsgRsp, // GameCommand = 1003 //登出
		int32(v1.GameCommand_OnReadyReq):       u.OnMsgRsp, // GameCommand = 1005 //准备
		int32(v1.GameCommand_OnSwitchTableReq): u.OnMsgRsp, // GameCommand = 1007 //换桌
		int32(v1.GameCommand_OnSceneReq):       u.OnMsgRsp, // GameCommand = 1009 //场景信息
		int32(v1.GameCommand_OnChatReq):        u.OnMsgRsp, // GameCommand = 1011 //聊天
		int32(v1.GameCommand_OnHostingReq):     u.OnMsgRsp, // GameCommand = 1013 //托管
		int32(v1.GameCommand_OnForwardReq):     u.OnMsgRsp, // GameCommand = 1015 //转发
		int32(v1.GameCommand_OnActionReq):      u.OnMsgRsp, // GameCommand = 1101 //玩家动作
		int32(v1.GameCommand_OnAutoCallReq):    u.OnMsgRsp, // GameCommand = 1103 //自动跟注
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
		log.Errorf("err:%v", err)
		return
	}
	u.client.Store(wsClient)
}

func (u *User) OnMsgPush(data []byte)            {}
func (u *User) OnMsgRsp(data []byte, code int32) {}

func (u *User) OnConnect(session *websocket.Session) {
	log.Infof("connect called. %q uid=%d", session.ID(), u.id)
	u.Request(v1.GameCommand_OnLoginReq, &v1.LoginReq{
		UserID: u.id,
	})
}

func (u *User) OnDisconnect(session *websocket.Session) {
	log.Infof("disconnect called. %q uid=%d", session.ID(), u.id)
}

func (u *User) Request(cmd v1.GameCommand, msg gproto.Message) {
	u.repo.GetLoop().Post(func() {
		wsClient := u.client.Load()
		if wsClient == nil {
			log.Warnf("wsClient is nil")
			return
		}
		if _, err := wsClient.Request(int32(cmd), msg); err != nil {
			log.Errorf("%v", err)
		}
	})
}
