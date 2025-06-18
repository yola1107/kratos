package table

import (
	v1 "github.com/yola1107/kratos/v2/ztest/api-server/api/helloworld/v1"
	"github.com/yola1107/kratos/v2/ztest/api-server/internal/biz/player"
	"github.com/yola1107/kratos/v2/ztest/api-server/internal/conf"
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

func (t *Table) OnActionReq(p *player.Player, in *v1.ActionReq, isTimeOut bool) (ok bool) {
	stage := t.stage.state
	if stage == conf.StWait || stage == conf.StReady || stage == conf.StWaitEnd || stage == conf.StEnd {
		return
	}

	if p == nil || !p.IsGaming() || len(t.GetCanActionPlayers()) <= 1 {
		return
	}

	switch in.Action {
	case v1.ACTION_SEE:
		t.handleSee(p, in)
	}

	return true
}

func (t *Table) handleSee(p *player.Player, in *v1.ActionReq) {
	if p.IsSee() {
		t.sendActionRsp(p, &v1.ActionRsp{Code: 1, Msg: "player seen", Action: in.Action})
		return
	}

	p.SetSee()

	t.broadcastActionRsp(p, in.Action)

	if p.GetChairID() == t.active {
		// 刷新定时器，通知活动玩家
		t.updateStage(conf.StAction)
		t.broadcastActivePlayerPush()
	} else {
		// 判断是否可以比牌
	}

}
