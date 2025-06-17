package biz

import (
	"context"

	"github.com/yola1107/kratos/v2/errors"
	"github.com/yola1107/kratos/v2/transport/websocket"
	"github.com/yola1107/kratos/v2/ztest/api-server/internal/biz/player"
	"github.com/yola1107/kratos/v2/ztest/api-server/internal/biz/table"
	"github.com/yola1107/kratos/v2/ztest/api-server/internal/model"
)

// SwapperInfo 玩家信息
type SwapperInfo struct {
	Error  *errors.Error
	Player *player.Player
	Table  *table.Table
}

func (uc *Usecase) Swapper(ctx context.Context) (r *SwapperInfo) {
	session := uc.GetSession(ctx)
	if session == nil {
		return &SwapperInfo{Error: model.ErrSessionNotFound}
	}

	p := uc.pm.GetBySessionID(session.ID())
	if p == nil {
		return &SwapperInfo{Error: model.ErrPlayerNotFound}
	}

	t := uc.tm.GetTable(p.GetTableID())
	if t == nil {
		return &SwapperInfo{Error: model.ErrTableNotFound}
	}

	return &SwapperInfo{
		Error:  nil,
		Player: p,
		Table:  t,
	}
}

func (uc *Usecase) GetSession(ctx context.Context) *websocket.Session {
	session, ok := ctx.Value(websocket.CtxSessionKey).(*websocket.Session)
	if !ok || session == nil {
		return nil
	}
	return session
}
