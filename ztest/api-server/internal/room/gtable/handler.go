package gtable

import (
	"github.com/yola1107/kratos/v2/ztest/api-server/internal/room/gplayer"
)

func (t *Table) OnExitGame(p *gplayer.Player, code int32, msg string) bool {
	return false
}

func (t *Table) OnForwardReq(ty int32, msg string) {
	t.broadcastForwardRsp(ty, msg)
	return
}

func (t *Table) OnSceneReq(p *gplayer.Player, isClient bool) {
	t.SendSceneInfo(p)
	return
}
