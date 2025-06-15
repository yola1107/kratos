package gplayer

import (
	"github.com/yola1107/kratos/v2/library/ext"
	. "github.com/yola1107/kratos/v2/ztest/api-server/internal/model"
)

// PlayerBaseData 结构体
type PlayerBaseData struct {
	UID           int64   // 用户ID
	TableID       int32   // 桌子ID
	ChairID       int32   // 椅子ID
	Money         float64 // 金币
	BMoney        float64 // 充值所得金币
	GMoney        float64 // 从bonus转换过来可提到BMoney额度
	ReserveMoney  float64 // 预留金币
	ChannelID     int32   // 渠道ID
	PayTotal      float64 // 总充值金额
	WithdrawTotal float64 // 玩家提现总金额
	Scbonus       float64 // 首充奖励金币
	Bonus         float64 // 奖励币
	ReduceBMoney  float64
	ReduceMoney   float64
	VIP           int32 // VIP等级
	NickName      string
	Avatar        string
	AvatarUrl     string

	PresentBet      float64 // 本次游戏中的所有下注
	PresentProfit   float64 // 本次游戏中的纯盈利
	PresentWinScore float64 // 本次游戏中的总赢
	PresentBoard    int32   // 局数

	TotalBoard   int32   // 此游戏的总局数
	TotalEarn    float64 // 此游戏的总赢
	TotalConsume float64 // 此游戏的总消耗

	AllTotalBoard int64 // 玩家游戏局数
}

func NewPlayerBaseData(uid int64) *PlayerBaseData {
	return &PlayerBaseData{}
}

func (p *Player) UseMoney(money float64) bool {
	return true
}

func (p *Player) GetMoney() float64 {
	return 0
}

func (p *Player) GetVipGrade() int32 {
	return p.baseData.VIP
}

func (p *Player) GetPlayerID() int64 {
	return p.baseData.UID
}

func (p *Player) SetTableID(tableID int32) {
	p.baseData.TableID = tableID
}

func (p *Player) GetTableID() (TableID int32) {
	return p.baseData.TableID
}

func (p *Player) SetChairID(ChairID int32) {
	p.baseData.ChairID = ChairID
	return
}

func (p *Player) GetChairID() (ChairID int32) {
	return p.baseData.ChairID
}

func (p *Player) GetNickName() string {
	return p.baseData.NickName
}

func (p *Player) GetAvatar() string {
	return p.baseData.Avatar
}

func (p *Player) GetAvatarUrl() string {
	return p.baseData.AvatarUrl
}

func (p *Player) GetIP() string {
	return ""
}
func (p *Player) SaveBaseDataToDB() {
}

func (p *Player) LoadBaseDataFromDB() {
}

// ToRedisMap 转为 Redis hash 的 map[string]string
func (b *PlayerBaseData) ToRedisMap() map[string]string {
	m := make(map[string]string)

	m[PlayerTableIDField] = ext.Int64ToStr(int64(b.TableID))
	m[PlayerUIDField] = ext.Int64ToStr(b.UID)
	m[PlayerMoneyField] = ext.Float64ToStr(b.Money)
	m[PlayerBMoneyField] = ext.Float64ToStr(b.BMoney)
	m[PlayerGMoneyField] = ext.Float64ToStr(b.GMoney)
	m[PlayerReserveMoneyField] = ext.Float64ToStr(b.ReserveMoney)
	m[PlayerChannelIDField] = ext.Int32ToStr(b.ChannelID)
	m[PlayerPayTotalField] = ext.Float64ToStr(b.PayTotal)
	m[PlayerWithdrawTotalField] = ext.Float64ToStr(b.WithdrawTotal)
	m[PlayerScbonusField] = ext.Float64ToStr(b.Scbonus)
	m[PlayerBonusField] = ext.Float64ToStr(b.Bonus)
	m[PlayerReduceBMoneyField] = ext.Float64ToStr(b.ReduceBMoney)
	m[PlayerReduceMoneyField] = ext.Float64ToStr(b.ReduceMoney)
	m[PlayerVIPField] = ext.Int32ToStr(b.VIP)
	m[PlayerNickNameField] = b.NickName
	m[PlayerAvatarField] = b.Avatar
	m[PlayerAvatarUrlField] = b.AvatarUrl
	m[PlayerPresentBetField] = ext.Float64ToStr(b.PresentBet)
	m[PlayerPresentProfitField] = ext.Float64ToStr(b.PresentProfit)
	m[PlayerPresentWinScoreField] = ext.Float64ToStr(b.PresentWinScore)
	m[PlayerPresentBoardField] = ext.Int32ToStr(b.PresentBoard)
	m[PlayerTotalBoardField] = ext.Int32ToStr(b.TotalBoard)
	m[PlayerTotalEarnField] = ext.Float64ToStr(b.TotalEarn)
	m[PlayerTotalConsumeField] = ext.Float64ToStr(b.TotalConsume)
	m[PlayerAllTotalBoardField] = ext.Int64ToStr(b.AllTotalBoard)

	return m
}

// FromRedisData 从 Redis hash 的 map[string]string 转为 baseData
func (b *PlayerBaseData) FromRedisData(data map[string]string) {
	b.TableID = ext.StrToInt32(data[PlayerTableIDField])
	b.UID = ext.StrToInt64(data[PlayerUIDField])
	b.Money = ext.StrToFloat64(data[PlayerMoneyField])
	b.BMoney = ext.StrToFloat64(data[PlayerBMoneyField])
	b.GMoney = ext.StrToFloat64(data[PlayerGMoneyField])
	b.ReserveMoney = ext.StrToFloat64(data[PlayerReserveMoneyField])
	b.ChannelID = ext.StrToInt32(data[PlayerChannelIDField])
	b.PayTotal = ext.StrToFloat64(data[PlayerPayTotalField])
	b.WithdrawTotal = ext.StrToFloat64(data[PlayerWithdrawTotalField])
	b.Scbonus = ext.StrToFloat64(data[PlayerScbonusField])
	b.Bonus = ext.StrToFloat64(data[PlayerBonusField])
	b.ReduceBMoney = ext.StrToFloat64(data[PlayerReduceBMoneyField])
	b.ReduceMoney = ext.StrToFloat64(data[PlayerReduceMoneyField])
	b.VIP = ext.StrToInt32(data[PlayerVIPField])
	b.NickName = data[PlayerNickNameField]
	b.Avatar = data[PlayerAvatarField]
	b.AvatarUrl = data[PlayerAvatarUrlField]
	b.PresentBet = ext.StrToFloat64(data[PlayerPresentBetField])
	b.PresentProfit = ext.StrToFloat64(data[PlayerPresentProfitField])
	b.PresentWinScore = ext.StrToFloat64(data[PlayerPresentWinScoreField])
	b.PresentBoard = ext.StrToInt32(data[PlayerPresentBoardField])
	b.TotalBoard = ext.StrToInt32(data[PlayerTotalBoardField])
	b.TotalEarn = ext.StrToFloat64(data[PlayerTotalEarnField])
	b.TotalConsume = ext.StrToFloat64(data[PlayerTotalConsumeField])
	b.AllTotalBoard = ext.StrToInt64(data[PlayerAllTotalBoardField])
}

//
// func SavePlayerToRedis(redisClient *redis.Client, key string, p *BaseData) error {
// 	data := p.ToRedisMap() // 转成 map[string]string
// 	return redisClient.HSet(context.Background(), key, data).Err()
// }
//
// func LoadPlayerFromRedis(redisClient *redis.Client, key string) (*BaseData, error) {
// 	data, err := redisClient.HGetAll(context.Background(), key).Result()
// 	if err != nil {
// 		return nil, err
// 	}
// 	if len(data) == 0 {
// 		return nil, nil // Redis 中该 key 不存在，返回 nil
// 	}
// 	p := &BaseData{}
// 	p.FromRedisData(data) // 反序列化到 BaseData
// 	return p, nil
// }
