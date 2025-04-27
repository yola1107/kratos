package service

import (
	"context"

	"github.com/yola1107/kratos/v2/errors"
	"github.com/yola1107/kratos/v2/transport/websocket"

	"github.com/yola1107/kratos/v2/ztest/api-server/internal/model"
	"github.com/yola1107/kratos/v2/ztest/api-server/internal/room/gplayer"
	"github.com/yola1107/kratos/v2/ztest/api-server/internal/room/gtable"
)

// 玩家信息
type stPlayerInfo struct {
	Error  *errors.Error
	Player *gplayer.Player
	Table  *gtable.Table
}

func (s *Service) swapper(ctx context.Context) (r *stPlayerInfo) {
	session := s.getSession(ctx)
	if session == nil {
		return &stPlayerInfo{Error: model.ErrSessionNotFound}
	}

	p := s.pm.GetPlayerBySessionID(session.ID())
	if p == nil {
		return &stPlayerInfo{Error: model.ErrPlayerNotFound}
	}

	t := s.tm.GetTable(p.GetTableID())
	if t == nil {
		return &stPlayerInfo{Error: model.ErrTableNotFound}
	}

	return &stPlayerInfo{
		Error:  nil,
		Player: p,
		Table:  t,
	}
}

func (s *Service) getSession(ctx context.Context) *websocket.Session {
	session, ok := ctx.Value(websocket.CtxSessionKey).(*websocket.Session)
	if !ok || session == nil {
		return nil
	}
	return session
}
