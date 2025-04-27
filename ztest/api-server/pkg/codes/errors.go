package codes

import (
	"github.com/yola1107/kratos/v2/errors"
)

var (
	ErrFail                 = errors.New(1, "", "Failed")
	ErrRoomClosed           = errors.New(2, "", "RoomClosed")
	ErrKickByError          = errors.New(3, "", "kick by error")
	ErrKickByBroke          = errors.New(4, "", "kick by broke")
	ErrKickByPasswordError  = errors.New(5, "", "kick by password error")
	ErrMoneyOverMaxLimit    = errors.New(6, "", "money over max limit")
	ErrMoneyBelowMinLimit   = errors.New(7, "", "money below min limit")
	ErrMoneyBelowBaseLimit  = errors.New(8, "", "money below base limit")
	ErrVipLimit             = errors.New(9, "", "vip limit")
	ErrTokenFail            = errors.New(10, "", "token fail")
	ErrSessionNotFound      = errors.New(11, "", "session not found")
	ErrPlayerNotFound       = errors.New(12, "", "player not found")
	ErrTableNotFound        = errors.New(13, "", "table not found")
	ErrSwitchTable          = errors.New(14, "", "switch table error")
	ErrNotEnoughTable       = errors.New(15, "", "not enough table")
	ErrExitTableFail        = errors.New(16, "", "exit table fail")
	ErrEnterTableFail       = errors.New(17, "", "enter table Fail")
	ErrCreatePlayerFail     = errors.New(18, "", "create player Fail")
	ErrPlayerAlreadyInTable = errors.New(19, "", "player already exists in table")
)
