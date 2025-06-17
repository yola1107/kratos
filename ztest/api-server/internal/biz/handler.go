package biz

import (
	"context"

	"github.com/yola1107/kratos/v2/library/ext"
	"github.com/yola1107/kratos/v2/log"
	v1 "github.com/yola1107/kratos/v2/ztest/api-server/api/helloworld/v1"
	"github.com/yola1107/kratos/v2/ztest/api-server/internal/biz/player"
	"github.com/yola1107/kratos/v2/ztest/api-server/internal/model"
)

func (uc *Usecase) OnLoginReq(ctx context.Context, in *v1.LoginReq) (*v1.LoginRsp, error) {
	if uc.pm.Has(in.UserID) {
		return uc.reconnect(ctx, in)
	}
	return uc.enterRoom(ctx, in)
}

func (uc *Usecase) reconnect(ctx context.Context, in *v1.LoginReq) (*v1.LoginRsp, error) {
	session := uc.GetSession(ctx)
	if session == nil {
		return nil, model.ErrSessionNotFound
	}
	uc.ws.Post(func() {
		p := uc.pm.GetByID(in.UserID)
		if p == nil {
			return
		}
		t := uc.tm.GetTable(p.GetTableID())
		if t == nil {
			return
		}
		p.UpdateSession(session)
		t.ReEnter(p)
	})
	return &v1.LoginRsp{}, nil
}

func (uc *Usecase) enterRoom(ctx context.Context, in *v1.LoginReq) (*v1.LoginRsp, error) {
	session := uc.GetSession(ctx)
	if session == nil {
		return nil, model.ErrSessionNotFound
	}
	p, err := uc.createPlayer(&player.Raw{
		ID:      in.UserID,
		IP:      session.GetRemoteIP(),
		Session: session,
	})
	if p == nil {
		log.Warnf("loginRoom. UserID(%+v) err=%v", in.UserID, err)
		return nil, err
	}
	// 条件限制
	if err := uc.tm.CanEnterRoom(p, in.Token, uc.rc.Game); err != nil {
		log.Warnf("loginRoom. UserID(%+v) err=%v", in.UserID, err)
		uc.LogoutGame(p, err.Code, err.Message) // 释放玩家
		return nil, err
	}
	uc.ws.Post(func() {
		if !uc.tm.ThrowInto(p) {
			log.Errorf("ThrowInto failed. pid:%d", in.UserID)
			uc.LogoutGame(p, 0, "throw into table failed")
			return
		}
	})
	return &v1.LoginRsp{}, nil
}

func (uc *Usecase) OnSwitchTableReq(rs *SwapperInfo) {
	ret := uc.tm.SwitchTable(rs.Player, uc.rc.Game)
	// 推送换桌消息
	rs.Player.SendSwitchTableRsp(ret)
}

func (uc *Usecase) createPlayer(raw *player.Raw) (*player.Player, error) {
	base, err := uc.repo.Load(context.Background(), raw.ID)
	if err != nil || base == nil {
		return nil, err
	}
	raw.Base = base
	p := player.New(raw)
	uc.pm.Add(p)
	return p, nil
}

func (uc *Usecase) LogoutGame(p *player.Player, code int32, msg string) {
	if p == nil {
		return
	}

	uc.pm.Remove(p.GetPlayerID())

	// 异步释放玩家
	go func() {
		defer ext.RecoverFromError(nil)

		if err := uc.SavePlayer(context.Background(), p); err != nil {
			log.Errorf("LogoutGame. err=%v", err)
		}
	}()
}
