package table

import (
	"github.com/golang/protobuf/proto"

	v1 "github.com/yola1107/kratos/v2/ztest/api-server/api/helloworld/v1"
	"github.com/yola1107/kratos/v2/ztest/api-server/internal/biz/player"
)

type RobotLogic struct {
	mTable *Table
}

func (r *RobotLogic) init(t *Table) {
	r.mTable = t
}

func (r *RobotLogic) OnMessage(p *player.Player, cmd v1.GameCommand, msg proto.Message) {

}

func (r *RobotLogic) canEnterRobot(p *player.Player) bool {
	if p == nil {
		return false
	}

	return true
}

func (r *RobotLogic) canExitRobot(p *player.Player) bool {
	if p == nil {
		return false
	}
	return true
}
