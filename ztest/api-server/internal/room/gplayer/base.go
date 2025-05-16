package gplayer

import (
	"github.com/yola1107/kratos/v2/ztest/api-server/internal/base"
	. "github.com/yola1107/kratos/v2/ztest/api-server/internal/model"
)

// baseData 结构体
type baseData struct {
	TableID       int32
	UID           int64
	Money         float64
	BMoney        float64
	GMoney        float64
	ReserveMoney  float64
	ChannelID     int
	PayTotal      float64
	WithdrawTotal float64
	Scbonus       float64
	Bonus         float64
	ReduceBMoney  float64
	ReduceMoney   float64
	VIP           int
	NickName      string
	Avatar        string
	AvatarUrl     string

	PresentBet      float64
	PresentProfit   float64
	PresentWinScore float64
	PresentBoard    int

	TotalBoard   int
	TotalEarn    float64
	TotalConsume float64

	AllTotalBoard int64
}

// ToRedisMap 转为 Redis hash 的 map[string]string
func (b *baseData) ToRedisMap() map[string]string {
	m := make(map[string]string)

	m[PlayerTableIDField] = base.Int64ToStr(int64(b.TableID))
	m[PlayerUIDField] = base.Int64ToStr(b.UID)
	m[PlayerMoneyField] = base.Float64ToStr(b.Money)
	m[PlayerBMoneyField] = base.Float64ToStr(b.BMoney)
	m[PlayerGMoneyField] = base.Float64ToStr(b.GMoney)
	m[PlayerReserveMoneyField] = base.Float64ToStr(b.ReserveMoney)
	m[PlayerChannelIDField] = base.IntToStr(b.ChannelID)
	m[PlayerPayTotalField] = base.Float64ToStr(b.PayTotal)
	m[PlayerWithdrawTotalField] = base.Float64ToStr(b.WithdrawTotal)
	m[PlayerScbonusField] = base.Float64ToStr(b.Scbonus)
	m[PlayerBonusField] = base.Float64ToStr(b.Bonus)
	m[PlayerReduceBMoneyField] = base.Float64ToStr(b.ReduceBMoney)
	m[PlayerReduceMoneyField] = base.Float64ToStr(b.ReduceMoney)
	m[PlayerVIPField] = base.IntToStr(b.VIP)
	m[PlayerNickNameField] = b.NickName
	m[PlayerAvatarField] = b.Avatar
	m[PlayerAvatarUrlField] = b.AvatarUrl
	m[PlayerPresentBetField] = base.Float64ToStr(b.PresentBet)
	m[PlayerPresentProfitField] = base.Float64ToStr(b.PresentProfit)
	m[PlayerPresentWinScoreField] = base.Float64ToStr(b.PresentWinScore)
	m[PlayerPresentBoardField] = base.IntToStr(b.PresentBoard)
	m[PlayerTotalBoardField] = base.IntToStr(b.TotalBoard)
	m[PlayerTotalEarnField] = base.Float64ToStr(b.TotalEarn)
	m[PlayerTotalConsumeField] = base.Float64ToStr(b.TotalConsume)
	m[PlayerAllTotalBoardField] = base.Int64ToStr(b.AllTotalBoard)

	return m
}

// FromRedisData 从 Redis hash 的 map[string]string 转为 baseData
func (b *baseData) FromRedisData(data map[string]string) {
	b.TableID = base.StrToInt32(data[PlayerTableIDField])
	b.UID = base.StrToInt64(data[PlayerUIDField])
	b.Money = base.StrToFloat64(data[PlayerMoneyField])
	b.BMoney = base.StrToFloat64(data[PlayerBMoneyField])
	b.GMoney = base.StrToFloat64(data[PlayerGMoneyField])
	b.ReserveMoney = base.StrToFloat64(data[PlayerReserveMoneyField])
	b.ChannelID = base.StrToInt(data[PlayerChannelIDField])
	b.PayTotal = base.StrToFloat64(data[PlayerPayTotalField])
	b.WithdrawTotal = base.StrToFloat64(data[PlayerWithdrawTotalField])
	b.Scbonus = base.StrToFloat64(data[PlayerScbonusField])
	b.Bonus = base.StrToFloat64(data[PlayerBonusField])
	b.ReduceBMoney = base.StrToFloat64(data[PlayerReduceBMoneyField])
	b.ReduceMoney = base.StrToFloat64(data[PlayerReduceMoneyField])
	b.VIP = base.StrToInt(data[PlayerVIPField])
	b.NickName = data[PlayerNickNameField]
	b.Avatar = data[PlayerAvatarField]
	b.AvatarUrl = data[PlayerAvatarUrlField]
	b.PresentBet = base.StrToFloat64(data[PlayerPresentBetField])
	b.PresentProfit = base.StrToFloat64(data[PlayerPresentProfitField])
	b.PresentWinScore = base.StrToFloat64(data[PlayerPresentWinScoreField])
	b.PresentBoard = base.StrToInt(data[PlayerPresentBoardField])
	b.TotalBoard = base.StrToInt(data[PlayerTotalBoardField])
	b.TotalEarn = base.StrToFloat64(data[PlayerTotalEarnField])
	b.TotalConsume = base.StrToFloat64(data[PlayerTotalConsumeField])
	b.AllTotalBoard = base.StrToInt64(data[PlayerAllTotalBoardField])
}

//func SavePlayerToRedis(redisClient *redis.Client, key string, p *BaseData) error {
//    data := p.ToRedisMap() // 转成 map[string]string
//    return redisClient.HSet(context.Background(), key, data).Err()
//}
//
//func LoadPlayerFromRedis(redisClient *redis.Client, key string) (*BaseData, error) {
//    data, err := redisClient.HGetAll(context.Background(), key).Result()
//    if err != nil {
//        return nil, err
//    }
//    if len(data) == 0 {
//        return nil, nil // Redis 中该 key 不存在，返回 nil
//    }
//    p := &BaseData{}
//    p.FromRedisData(data) // 反序列化到 BaseData
//    return p, nil
//}
