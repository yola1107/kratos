package gtable

import (
	"github.com/yola1107/kratos/v2/ztest/api-server/internal/room/gplayer"
)

func (t *Table) OnExitGame(p *gplayer.Player, code int32, msg string) bool {
	return false
}

func (t *Table) OnForwardReq(ty int32, msg string) {
	return
}

func (t *Table) OnSceneReq(ty int32, msg string) {
	return
}
