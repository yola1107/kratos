package iface

//
// import (
// 	"github.com/yola1107/kratos/v2/library/work"
// 	"github.com/yola1107/kratos/v2/ztest/api-server/internal/conf"
// )
//
// /*
//
// 	解决循环依赖（cyclic dependency）问题：
// 	service.room 需要用到 table_manager 和 player_manager 实例；
// 	而 table 和 player 又会在逻辑上回调 room 中的函数（例如广播、解散房间、玩家离开等）。
// 	推荐方案：引入接口 + 回调抽象 + 初始化依赖注入
//
//
// 	[ room ] ─────────┬────────→ 使用接口（TableManager / PlayerManager）
// 					  │
// 					  ↓
// 	  定义 IRoomRepo 接口 ←────── [ table / player ]
// 							↑
// 					  注入回调实现（room.Room）
// */
//
// // IRoomRepo room提供callback给table、player等使用
// type IRoomRepo interface {
// 	GetLoop() work.ITaskLoop
// 	GetTimer() work.ITaskScheduler
// 	GetRoomConfig() *conf.Room
// 	OnPlayerLeave(playerID string)
// 	OnTableEvent(tableID string, evt string)
// }
//
