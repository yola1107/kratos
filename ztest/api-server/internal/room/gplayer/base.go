package gplayer

type baseData struct {
	TableID       int32
	UID           int64   // 用户ID
	Money         float64 // 金币
	BMoney        float64 // 充值所得金币
	GMoney        float64 // 从bonus转换过来可提到BMoney额度
	ReserveMoney  float64 // 预留金币
	ChannelID     int     // 渠道ID
	PayTotal      float64 // 总充值金额
	WithdrawTotal float64 // 玩家提现总金额
	Scbonus       float64 // 首充奖励金币
	Bonus         float64 // 奖励币
	ReduceBMoney  float64
	ReduceMoney   float64
	VIP           int // VIP等级
	NickName      string
	Avatar        string
	AvatarUrl     string

	PresentBet      float64 //本次游戏中的所有下注
	PresentProfit   float64 //本次游戏中的纯盈利
	PresentWinScore float64 //本次游戏中的总赢
	PresentBoard    int     //局数

	TotalBoard   int     // 此游戏的总局数
	TotalEarn    float64 // 此游戏的总赢
	TotalConsume float64 // 此游戏的总消耗

	AllTotalBoard int64 // 玩家游戏局数
}
