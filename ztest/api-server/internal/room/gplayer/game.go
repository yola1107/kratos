package gplayer

type PlayerGameData struct {
	SID          int32   // 椅子ID
	IsReady      bool    // 是否准备
	Bet          float64 // 投注
	Status       int     // 0 betting 1 fold 2 lose
	LastOperator int     // 上一次操作
	See          bool    // 是否看牌
	Card         []int32 // 用户手牌

	PlayCount  int     // 玩的回合数
	SeeRound   int     // 看牌回合数
	StartMoney float64 // 局数开始时的金币
	IdleCount  int     // 超时次数

	CompareSeats []int // 比牌椅子号
	IsAllCompare bool  // 是否参与所有比牌

	AutoCall int  // 是否自动跟注 0：未开启自动跟注 1：开启了自动跟注
	Paying   bool // 支付中
}
