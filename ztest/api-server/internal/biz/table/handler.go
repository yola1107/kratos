package table

import (
	v1 "github.com/yola1107/kratos/v2/ztest/api-server/api/helloworld/v1"
	"github.com/yola1107/kratos/v2/ztest/api-server/internal/biz/player"
)

func (t *Table) OnExitGame(p *player.Player, code int32, msg string) bool {
	if !t.ThrowOff(p, false) {
		return false
	}
	t.repo.LogoutGame(p, code, "")
	return false
}

func (t *Table) OnSceneReq(p *player.Player, isClient bool) {
	t.SendSceneInfo(p)
	return
}

func (t *Table) OnReadyReq(p *player.Player, isReady bool) bool {
	return true
}

func (t *Table) OnChatReq(p *player.Player, in *v1.ChatReq) bool {
	return true
}

func (t *Table) OnHosting(p *player.Player, isHosting bool) bool {
	return true
}

func (t *Table) OnAutoCallReq(p *player.Player, autoCall bool) bool {
	return true
}

func (t *Table) OnActionReq(p *player.Player, in *v1.ActionReq) bool {
	return true
}
