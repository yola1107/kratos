package data

import (
	"context"

	"github.com/yola1107/kratos/v2/log"
	"github.com/yola1107/kratos/v2/ztest/api-server/internal/biz/player"
)

func (r *dataRepo) SavePlayer(ctx context.Context, p *player.BaseData) error {
	// 将 player.BaseData 转成 DO（数据库模型），写入数据库
	log.Infof("保存玩家 %d 到数据库: %+v\n", p.UID, p.NickName)
	return nil
}

func (r *dataRepo) LoadPlayer(ctx context.Context, playerID int64) (*player.BaseData, error) {
	// 从数据库读取并还原为 player.BaseData
	log.Infof("从数据库加载玩家 %d\n", playerID)
	return &player.BaseData{
		UID:      playerID,
		NickName: "测试玩家",
	}, nil
}

// func (p *Player) SaveBaseDataToDB() {
// }
//
// func (p *Player) LoadBaseDataFromDB() {
// }

// // ToRedisMap 转为 Redis hash 的 map[string]string
// func (b *BaseData) ToRedisMap() map[string]string {
// 	m := make(map[string]string)
//
// 	m[PlayerTableIDField] = ext.Int64ToStr(int64(b.TableID))
// 	m[PlayerUIDField] = ext.Int64ToStr(b.UID)
// 	m[PlayerMoneyField] = ext.Float64ToStr(b.Money)
// 	m[PlayerBMoneyField] = ext.Float64ToStr(b.BMoney)
// 	m[PlayerGMoneyField] = ext.Float64ToStr(b.GMoney)
// 	m[PlayerReserveMoneyField] = ext.Float64ToStr(b.ReserveMoney)
// 	m[PlayerChannelIDField] = ext.Int32ToStr(b.ChannelID)
// 	m[PlayerPayTotalField] = ext.Float64ToStr(b.PayTotal)
// 	m[PlayerWithdrawTotalField] = ext.Float64ToStr(b.WithdrawTotal)
// 	m[PlayerScbonusField] = ext.Float64ToStr(b.Scbonus)
// 	m[PlayerBonusField] = ext.Float64ToStr(b.Bonus)
// 	m[PlayerReduceBMoneyField] = ext.Float64ToStr(b.ReduceBMoney)
// 	m[PlayerReduceMoneyField] = ext.Float64ToStr(b.ReduceMoney)
// 	m[PlayerVIPField] = ext.Int32ToStr(b.VIP)
// 	m[PlayerNickNameField] = b.NickName
// 	m[PlayerAvatarField] = b.Avatar
// 	m[PlayerAvatarUrlField] = b.AvatarUrl
// 	m[PlayerPresentBetField] = ext.Float64ToStr(b.PresentBet)
// 	m[PlayerPresentProfitField] = ext.Float64ToStr(b.PresentProfit)
// 	m[PlayerPresentWinScoreField] = ext.Float64ToStr(b.PresentWinScore)
// 	m[PlayerPresentBoardField] = ext.Int32ToStr(b.PresentBoard)
// 	m[PlayerTotalBoardField] = ext.Int32ToStr(b.TotalBoard)
// 	m[PlayerTotalEarnField] = ext.Float64ToStr(b.TotalEarn)
// 	m[PlayerTotalConsumeField] = ext.Float64ToStr(b.TotalConsume)
// 	m[PlayerAllTotalBoardField] = ext.Int64ToStr(b.AllTotalBoard)
//
// 	return m
// }

// // FromRedisData 从 Redis hash 的 map[string]string 转为 baseData
// func (b *BaseData) FromRedisData(data map[string]string) {
// 	b.TableID = ext.StrToInt32(data[PlayerTableIDField])
// 	b.UID = ext.StrToInt64(data[PlayerUIDField])
// 	b.Money = ext.StrToFloat64(data[PlayerMoneyField])
// 	b.BMoney = ext.StrToFloat64(data[PlayerBMoneyField])
// 	b.GMoney = ext.StrToFloat64(data[PlayerGMoneyField])
// 	b.ReserveMoney = ext.StrToFloat64(data[PlayerReserveMoneyField])
// 	b.ChannelID = ext.StrToInt32(data[PlayerChannelIDField])
// 	b.PayTotal = ext.StrToFloat64(data[PlayerPayTotalField])
// 	b.WithdrawTotal = ext.StrToFloat64(data[PlayerWithdrawTotalField])
// 	b.Scbonus = ext.StrToFloat64(data[PlayerScbonusField])
// 	b.Bonus = ext.StrToFloat64(data[PlayerBonusField])
// 	b.ReduceBMoney = ext.StrToFloat64(data[PlayerReduceBMoneyField])
// 	b.ReduceMoney = ext.StrToFloat64(data[PlayerReduceMoneyField])
// 	b.VIP = ext.StrToInt32(data[PlayerVIPField])
// 	b.NickName = data[PlayerNickNameField]
// 	b.Avatar = data[PlayerAvatarField]
// 	b.AvatarUrl = data[PlayerAvatarUrlField]
// 	b.PresentBet = ext.StrToFloat64(data[PlayerPresentBetField])
// 	b.PresentProfit = ext.StrToFloat64(data[PlayerPresentProfitField])
// 	b.PresentWinScore = ext.StrToFloat64(data[PlayerPresentWinScoreField])
// 	b.PresentBoard = ext.StrToInt32(data[PlayerPresentBoardField])
// 	b.TotalBoard = ext.StrToInt32(data[PlayerTotalBoardField])
// 	b.TotalEarn = ext.StrToFloat64(data[PlayerTotalEarnField])
// 	b.TotalConsume = ext.StrToFloat64(data[PlayerTotalConsumeField])
// 	b.AllTotalBoard = ext.StrToInt64(data[PlayerAllTotalBoardField])
// }

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
