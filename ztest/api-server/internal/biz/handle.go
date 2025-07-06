package biz

import (
	"context"
	"fmt"

	"github.com/yola1107/kratos/v2/library/ext"
	"github.com/yola1107/kratos/v2/library/work"
	"github.com/yola1107/kratos/v2/log"
	"github.com/yola1107/kratos/v2/transport/websocket"
	v1 "github.com/yola1107/kratos/v2/ztest/api-server/api/helloworld/v1"
	"github.com/yola1107/kratos/v2/ztest/api-server/internal/biz/player"
	"github.com/yola1107/kratos/v2/ztest/api-server/internal/biz/table"
	"github.com/yola1107/kratos/v2/ztest/api-server/internal/conf"
	"github.com/yola1107/kratos/v2/ztest/api-server/pkg/codes"
)

// GetLoop 获取任务队列
func (uc *Usecase) GetLoop() work.ITaskLoop {
	return uc.loop
}

// GetTimer 获取定时器
func (uc *Usecase) GetTimer() work.ITaskScheduler {
	return uc.timer
}

// GetRoomConfig 获取房间配置
func (uc *Usecase) GetRoomConfig() *conf.Room {
	return uc.rc
}

// GetTableList 获取桌子列表
func (uc *Usecase) GetTableList() []*table.Table {
	return uc.tm.GetTableList()
}

// OnLoginReq .
func (uc *Usecase) OnLoginReq(ctx context.Context, in *v1.LoginReq) (*v1.LoginRsp, error) {
	if uc.pm.Has(in.UserID) {
		return uc.reconnect(ctx, in)
	}
	return uc.enterRoom(ctx, in)
}

func (uc *Usecase) reconnect(ctx context.Context, in *v1.LoginReq) (*v1.LoginRsp, error) {
	session := uc.GetSession(ctx)
	if session == nil {
		return nil, codes.ErrSessionNotFound
	}

	uc.loop.Post(func() {
		p := uc.pm.GetByID(in.UserID)
		if p == nil {
			return
		}
		if t := uc.tm.GetTable(p.GetTableID()); t != nil {
			p.UpdateSession(session)
			t.ReEnter(p)
		}
	})
	return &v1.LoginRsp{}, nil
}

func (uc *Usecase) enterRoom(ctx context.Context, in *v1.LoginReq) (*v1.LoginRsp, error) {
	session := uc.GetSession(ctx)
	if session == nil {
		return nil, codes.ErrSessionNotFound
	}

	raw := &player.Raw{
		ID:      in.UserID,
		Session: session,
	}
	p, err := uc.createPlayer(raw)
	if err != nil {
		log.Warnf("create player failed. uid=%d err=%+v", in.UserID, err)
		return nil, err
	}

	if err := uc.tm.CanEnterRoom(p, in.Token, uc.rc.Game); err != nil {
		log.Warnf("room limit. uid=%d err=%v", in.UserID, err)
		uc.LogoutGame(p, err.Code, err.Message)
		return nil, err
	}

	uc.loop.Post(func() {
		if tableID := p.GetTableID(); tableID > 0 {
			uc.log.Warnf("enter failed. aleady in table. uid=%d tableID=%v", in.UserID, tableID)
			uc.LogoutGame(p, codes.ErrPlayerAlreadyInTable.Code, "already in table")
			return
		}
		if ok := uc.tm.ThrowInto(p); !ok {
			uc.log.Errorf("throw into failed. uid=%d ", in.UserID)
			uc.LogoutGame(p, codes.ErrEnterTableFail.Code, "throw into table failed")
			return
		}
	})

	return &v1.LoginRsp{}, nil
}

// OnSwitchTableReq .
func (uc *Usecase) OnSwitchTableReq(info *SwapperInfo) {
	code, msg := int32(0), ""
	if e := uc.tm.SwitchTable(info.Player, uc.rc.Game); e != nil {
		code, msg = e.Code, e.Message
	}
	info.Player.SendSwitchTableRsp(code, msg)
}

// CreateRobot .
func (uc *Usecase) CreateRobot(raw *player.Raw) (*player.Player, error) {
	// ctx := context.Background()
	// if !uc.repo.ExistPlayer(ctx, raw.ID) {
	base := &player.BaseData{
		UID:       raw.ID,
		VIP:       0,
		NickName:  fmt.Sprintf("robot_%d", raw.ID),
		Avatar:    fmt.Sprintf("robot_avatar_%d", raw.ID),
		AvatarUrl: fmt.Sprintf("robot_avatar_%d", raw.ID),
		Money:     ext.RandFloat(uc.rc.Game.MinMoney, uc.rc.Game.MaxMoney),
	}
	// if err := uc.repo.SavePlayer(context.Background(), base); err != nil {
	// 	return nil, err
	// }
	raw.BaseData = base
	p := player.New(raw)
	return p, nil
	// }
	// return uc.createPlayer(raw)
}

func (uc *Usecase) createPlayer(raw *player.Raw) (*player.Player, error) {
	// 获取数据库数据
	base, err := uc.repo.LoadPlayer(context.Background(), raw.ID)
	if err != nil {
		return nil, err
	}
	if base == nil {
		return nil, codes.ErrCreatePlayerFail
	}

	raw.BaseData = base
	p := player.New(raw)
	if !raw.IsRobot {
		uc.pm.Add(p)
	}
	log.Debugf("create player success. p:%+v ", p.Desc())
	return p, nil
}

func (uc *Usecase) Disconnect(session *websocket.Session) {
	if session == nil {
		return
	}

	p := uc.pm.GetBySessionID(session.ID())
	if p == nil {
		return
	}

	t := uc.tm.GetTable(p.GetTableID())
	if t == nil {
		uc.LogoutGame(p, codes.ErrTableNotFound.Code, fmt.Sprintf("disconnect. table is nil. pid:%d", p.GetPlayerID()))
		return
	}

	p.UpdateSession(nil)
	t.OnOffline(p)
}

// LogoutGame .
func (uc *Usecase) LogoutGame(p *player.Player, code int32, msg string) {
	if p == nil {
		return
	}

	uid := p.GetPlayerID()
	if p.IsRobot() {
		uc.rm.Leave(uid)
		return
	} else {
		uc.pm.Remove(uid)
	}

	log.Infof("logoutGame. p:%+v code=%d msg=%q", p.Desc(), code, msg)

	// 异步释放玩家
	uc.loop.Post(func() {
		// 数据入库
		baseData := *(p.GetBaseData()) // 复制一份

		if err := uc.repo.SavePlayer(context.Background(), &baseData); err != nil {
			uc.log.Warnf("save player failed: uid=%d %v", uid, err)
		}

		// 通知并清理
		p.LogoutGame(code, msg)
	})

	// // 异步释放玩家
	// go func() {
	// 	defer ext.RecoverFromError(nil)
	//
	// 	// 数据入库
	// 	baseData := *(p.GetBaseData()) // 复制一份
	// 	if err := uc.repo.SavePlayer(context.Background(), &baseData); err != nil {
	// 		uc.log.Warnf("save player failed: %v", err)
	// 	}
	//
	// 	// 通知并清理
	// 	p.LogoutGame(code, msg)
	// }()
}
