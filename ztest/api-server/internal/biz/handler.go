package biz

import (
	"context"

	"github.com/yola1107/kratos/v2/library/ext"
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
		return nil, model.ErrSessionNotFound
	}

	raw := &player.Raw{
		ID:      in.UserID,
		Session: session,
	}
	p, err := uc.createPlayer(raw)
	if err != nil {
		uc.log.Warnf("createPlayer failed: %v", err)
		return nil, err
	}

	if err := uc.tm.CanEnterRoom(p, in.Token, uc.rc.Game); err != nil {
		uc.log.Warnf("canEnterRoom failed for user %d: %v", in.UserID, err)
		uc.LogoutGame(p, err.Code, err.Message)
		return nil, err
	}

	uc.ws.Post(func() {
		if ok := uc.tm.ThrowInto(p); !ok {
			uc.log.Errorf("ThrowInto failed. UserID(%d)", in.UserID)
			uc.LogoutGame(p, 0, "throw into table failed")
		}
	})

	return &v1.LoginRsp{}, nil
}

func (uc *Usecase) OnSwitchTableReq(info *SwapperInfo) {
	result := uc.tm.SwitchTable(info.Player, uc.rc.Game)
	info.Player.SendSwitchTableRsp(result)
}

func (uc *Usecase) createPlayer(raw *player.Raw) (*player.Player, error) {
	base, err := uc.repo.Load(context.Background(), raw.ID)
	if err != nil || base == nil {
		return nil, err
	}
	raw.BaseData = base

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
			uc.log.Errorf("SavePlayer failed on logout. UserID(%d) err=%v", p.GetPlayerID(), err)
		}
	}()
}
