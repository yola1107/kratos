package table

import (
	"github.com/yola1107/kratos/v2/ztest/api-server/internal/biz/player"
)

func (t *Table) OnExitGame(p *player.Player, code int32, msg string) bool {
	if !t.ThrowOff(p, false) {
		return false
	}

	// playermgr.LogoutGame(p, code, "")

	t.repo.LogoutGame(p, code, "")
	return false
}

func (t *Table) OnForwardReq(ty int32, msg string) {
	t.BroadcastForwardRsp(ty, msg)
	return
}

func (t *Table) OnSceneReq(p *player.Player, isClient bool) {
	t.SendSceneInfo(p)
	return
}
