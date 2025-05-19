package gplayer

type gameData struct {
	// 游戏过程数据
	ChairID      int32   // 椅子ID
	IsReady      bool    // 是否准备
	Bet          float64 // 投注
	Status       int32   // 0 betting 1 fold 2 lose
	LastOperator int32   // 上一次操作
	See          bool    // 是否看牌
	//Card         util.HandCard // 用户手牌

	PlayCount  int32   // 玩的回合数
	SeeRound   int32   // 看牌回合数
	StartMoney float64 // 局数开始时的金币
	IdleCount  int32   // 超时次数

	CompareSeats []int // 比牌椅子号
	IsAllCompare bool  // 是否参与所有比牌

	AutoCall int32 //是否自动跟注 0：未开启自动跟注 1：开启了自动跟注
	Paying   bool  //支付中

	enterHistory map[int]bool //进桌历史
}
