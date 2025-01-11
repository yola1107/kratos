package proto

const (
	// OpProtoReady proto ready
	OpProtoReady = int32(1)
	// OpProtoFinish proto finish
	OpProtoFinish = int32(2)
)

type Pattern byte

const (
	_              Pattern = iota
	NODE_TYPE_HALL         //客户端大厅
	NODE_TYPE_PS           //广场
	NODE_TYPE_MC           //比赛调度程序，工程名MatchCtrl
	NODE_TYPE_GS           //比赛桌子服务器，工程名已改为TableServer，命名为GS为了符合大家的习惯称呼
	NODE_TYPE_CC           //比赛管理中心，工程名预定为ControlCenter
	NODE_TYPE_CS           //远程控制服务，工程名预定为CtrlService，不一定会用到SG，先定义好占个名额
	NODE_TYPE_SG           //ServerGate
	NODE_TYPE_IS           //信息服务中心，工程名InformationService，包括公告、好友、聊天、短信报警、web通信等模块
	NODE_TYPE_DBE          //DBEngineCenter，不一定会用到，但系统中有此服务程序，定义一个占位置
	NODE_TYPE_CHAT         //预留聊天服务器，现在还没有聊天服务器。ServerGate直接转给聊天MC，MC再转给ServerGate实现世界聊天
	NODE_TYPE_GD   = 39    //game message
)

const (
	AuthReq = 1000
	CMD_GAME_DATA = 1039
	CMD_VOICE_DATA = 10147
	HallPingReq   = 10230
	HallPingRsp   = 10231
)
