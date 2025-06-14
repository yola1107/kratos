package model

import (
	"github.com/yola1107/kratos/v2/errors"
)

var (
	SUCCESS                = int32(0)
	Fail                   = int32(1)
	ErrFail                = errors.New(1, "", "Failed")
	ErrRoomClosed          = errors.New(2, "", "RoomClosed")
	ErrKickByError         = errors.New(3, "", "kick by error")
	ErrKickByBroke         = errors.New(4, "", "kick by broke")
	ErrKickByPasswordError = errors.New(5, "", "kick by password error")
	ErrMoneyOverMaxLimit   = errors.New(6, "", "money over max limit")
	ErrMoneyBelowMinLimit  = errors.New(7, "", "money below min limit")
	ErrMoneyBelowBaseLimit = errors.New(8, "", "money below base limit")
	ErrVipLimit            = errors.New(9, "", "vip limit")
	ErrTokenFail           = errors.New(10, "", "token fail")
	ErrSessionNotFound     = errors.New(11, "", "session not found")
	ErrPlayerNotFound      = errors.New(12, "", "player not found")
	ErrTableNotFound       = errors.New(13, "", "table not found")
)
