package biz

import (
	"context"

	"github.com/yola1107/kratos/v2/transport/websocket"
	"github.com/yola1107/kratos/v2/ztest/game/ludo/internal/biz/player"
	"github.com/yola1107/kratos/v2/ztest/game/ludo/internal/biz/table"
	"github.com/yola1107/kratos/v2/ztest/game/ludo/pkg/codes"
)

// SwapperInfo 玩家信息
type SwapperInfo struct {
	Code   int32
	Msg    string
	Player *player.Player
	Table  *table.Table
}

func (uc *Usecase) Swapper(ctx context.Context) (r *SwapperInfo) {
	session := uc.GetSession(ctx)
	if session == nil {
		return &SwapperInfo{Code: codes.SESSION_NOT_FOUND}
	}

	p := uc.pm.GetBySessionID(session.ID())
	if p == nil {
		return &SwapperInfo{Code: codes.PLAYER_NOT_FOUND}
	}

	t := uc.tm.GetTable(p.GetTableID())
	if t == nil {
		return &SwapperInfo{Code: codes.TABLE_NOT_FOUND}
	}

	return &SwapperInfo{
		Code:   0,
		Msg:    "",
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
