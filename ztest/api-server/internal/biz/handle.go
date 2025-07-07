package biz

import (
	"context"
	"fmt"
	"time"

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

const (
	_maxRetryCount = 10                    // 最大重试次数
	_retryInterval = 50 * time.Millisecond // 每次重试间隔
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
		return &v1.LoginRsp{}, nil
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
		return &v1.LoginRsp{}, nil
	}

	raw := &player.Raw{
		ID:       in.UserID,
		Session:  session,
		BaseData: &player.BaseData{UID: in.UserID},
	}
	p, err := uc.createPlayer(raw)
	if err != nil {
		log.Errorf("create player failed. uid=%d err=%q", in.UserID, err)
		return &v1.LoginRsp{}, nil
	}

	if code, msg := uc.tm.CanEnterRoom(p, in.Token, uc.rc.Game); code != codes.SUCCESS {
		log.Errorf("room limit. uid=%d code=%d msg=%q", in.UserID, code, msg)
		uc.LogoutGame(p, code, msg)
		return &v1.LoginRsp{}, nil
	}

	uc.loop.Post(func() {
		if tableID := p.GetTableID(); tableID > 0 {
			uc.log.Errorf("enter failed. aleady in table. uid=%d tableID=%v", in.UserID, tableID)
			uc.LogoutGame(p, codes.PLAYER_ALREADY_IN_TABLE, "PLAYER_ALREADY_IN_TABLE")
			return
		}
		if code, msg := uc.tryThrowInto(p, _maxRetryCount, _retryInterval); code != codes.SUCCESS {
			uc.log.Errorf("throw into failed. uid=%d code=%d msg=%v", in.UserID, code, msg)
			uc.LogoutGame(p, code, msg)
			return
		}
	})

	return &v1.LoginRsp{}, nil
}

func (uc *Usecase) tryThrowInto(p *player.Player, maxRetries int, interval time.Duration) (code int32, msg string) {
	for i := 0; i <= maxRetries; i++ {
		code, msg = uc.tm.ThrowInto(p)
		if code == codes.SUCCESS || code == codes.PLAYER_INVALID {
			break
		}
		time.Sleep(interval)
	}
	return code, msg
}

// OnSwitchTableReq .
func (uc *Usecase) OnSwitchTableReq(info *SwapperInfo) {
	code, msg := uc.tm.SwitchTable(info.Player, uc.rc.Game)
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
		Avatar:    fmt.Sprintf("avatar_%d", raw.ID),
		AvatarUrl: fmt.Sprintf("avatar_%d", raw.ID),
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

var _OpenTest = true

func (uc *Usecase) createPlayer(raw *player.Raw) (*player.Player, error) {
	var (
		err  error
		base *player.BaseData
		p    = player.New(raw)
	)

	// 获取数据库数据
	if base, err = uc.repo.LoadPlayer(context.Background(), raw.ID); err != nil || base == nil {
		if _OpenTest {
			base = &player.BaseData{
				UID:       raw.ID,
				VIP:       0,
				NickName:  fmt.Sprintf("user_%d", raw.ID),
				Avatar:    fmt.Sprintf("avatar_%d", raw.ID%15),
				AvatarUrl: fmt.Sprintf("avatar_%d", raw.ID%15),
				Money:     float64(int64(ext.RandFloat(uc.rc.Game.MinMoney, uc.rc.Game.MaxMoney))),
			}

		} else {
			p.SendLoginRsp(codes.CREATE_PLAYER_FAIL, fmt.Sprintf("err=%v", err))
			return nil, err
		}
	}

	if _OpenTest {
		base.Money = float64(int64(ext.RandFloat(uc.rc.Game.MinMoney, uc.rc.Game.MaxMoney)))
	}

	p.SetBaseData(base)
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
		session.Close(false) //
		return
	}

	t := uc.tm.GetTable(p.GetTableID())
	if t == nil {
		uc.LogoutGame(p, codes.TABLE_NOT_FOUND, "disconnect by can not find table")
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
}
