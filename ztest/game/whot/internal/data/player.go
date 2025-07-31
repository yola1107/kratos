package data

import (
	"context"
	"errors"
	"fmt"

	"github.com/yola1107/kratos/v2/library/xgo"
	"github.com/yola1107/kratos/v2/ztest/game/whot/internal/biz/player"
	. "github.com/yola1107/kratos/v2/ztest/game/whot/pkg/xredis"
)

var (
	errRedisNil = errors.New("redis no exist player")
)

var allBaseDataFields = []string{
	PlayerUIDField,
	PlayerVIPField,
	PlayerNickNameField,
	PlayerAvatarField,
	PlayerAvatarUrlField,
	PlayerMoneyField,
	PlayerChannelIDField,
	PlayerPayTotalField,
	PlayerWithdrawTotalField,
	PlayerPresentBetField,
	PlayerPresentProfitField,
	PlayerPresentWinScoreField,
	PlayerPresentBoardField,
	PlayerTotalBoardField,
	PlayerTotalEarnField,
	PlayerTotalConsumeField,
	PlayerAllTotalBoardField,
}

func GetPlayerKey(uid int64) string {
	return fmt.Sprintf("account:user:%v", uid)
}

func (r *dataRepo) SavePlayer(ctx context.Context, base *player.BaseData) error {
	key := GetPlayerKey(base.UID)
	err := r.data.redis.HMSet(ctx, key, ToRedisMap(base)).Err()
	if err != nil {
		return err
	}
	return nil
}

func (r *dataRepo) ExistPlayer(ctx context.Context, uid int64) bool {
	key := GetPlayerKey(uid)
	v, err := r.data.redis.Exists(ctx, key).Result()
	return v != 0 && err == nil
}

// LoadPlayer ==> BaseUserInfoGet
func (r *dataRepo) LoadPlayer(ctx context.Context, uid int64) (*player.BaseData, error) {
	key := GetPlayerKey(uid)

	v, err := r.data.redis.Exists(ctx, key).Result()
	if err != nil {
		return nil, err
	}
	if v == 0 {
		return nil, errRedisNil
	}

	values, err := r.data.redis.HMGet(ctx, key, allBaseDataFields...).Result()
	if err != nil {
		return nil, err
	}
	if len(values) == 0 {
		return nil, errRedisNil
	}
	return FromRedisData(uid, AddList(allBaseDataFields, values)), nil
}

func AddList(keys []string, values []any) map[string]string {
	p := map[string]string{}
	for i, v := range values {
		p[keys[i]] = fmt.Sprintf("%v", v)
	}
	return p
}

func FromRedisData(uid int64, data map[string]string) *player.BaseData {
	b := &player.BaseData{}

	b.UID = xgo.StrToInt64(data[PlayerUIDField])
	b.VIP = xgo.StrToInt32(data[PlayerVIPField])
	b.NickName = data[PlayerNickNameField]
	b.Avatar = data[PlayerAvatarField]
	b.AvatarUrl = data[PlayerAvatarUrlField]
	b.Money = xgo.StrToFloat64(data[PlayerMoneyField])
	b.ChannelID = xgo.StrToInt32(data[PlayerChannelIDField])
	b.PayTotal = xgo.StrToFloat64(data[PlayerPayTotalField])
	b.WithdrawTotal = xgo.StrToFloat64(data[PlayerWithdrawTotalField])
	b.PresentBet = xgo.StrToFloat64(data[PlayerPresentBetField])
	b.PresentProfit = xgo.StrToFloat64(data[PlayerPresentProfitField])
	b.PresentWinScore = xgo.StrToFloat64(data[PlayerPresentWinScoreField])
	b.PresentBoard = xgo.StrToInt32(data[PlayerPresentBoardField])
	b.TotalBoard = xgo.StrToInt32(data[PlayerTotalBoardField])
	b.TotalEarn = xgo.StrToFloat64(data[PlayerTotalEarnField])
	b.TotalConsume = xgo.StrToFloat64(data[PlayerTotalConsumeField])
	b.AllTotalBoard = xgo.StrToInt64(data[PlayerAllTotalBoardField])

	b.UID = uid
	return b
}

// ToRedisMap 转为 Redis hash 的 map[string]string
func ToRedisMap(b *player.BaseData) map[string]string {
	m := make(map[string]string)
	m[PlayerUIDField] = xgo.Int64ToStr(b.UID)
	m[PlayerMoneyField] = xgo.Float64ToStr(b.Money)
	m[PlayerChannelIDField] = xgo.Int32ToStr(b.ChannelID)
	m[PlayerPayTotalField] = xgo.Float64ToStr(b.PayTotal)
	m[PlayerWithdrawTotalField] = xgo.Float64ToStr(b.WithdrawTotal)
	m[PlayerVIPField] = xgo.Int32ToStr(b.VIP)
	m[PlayerNickNameField] = b.NickName
	m[PlayerAvatarField] = b.Avatar
	m[PlayerAvatarUrlField] = b.AvatarUrl
	m[PlayerPresentBetField] = xgo.Float64ToStr(b.PresentBet)
	m[PlayerPresentProfitField] = xgo.Float64ToStr(b.PresentProfit)
	m[PlayerPresentWinScoreField] = xgo.Float64ToStr(b.PresentWinScore)
	m[PlayerPresentBoardField] = xgo.Int32ToStr(b.PresentBoard)
	m[PlayerTotalBoardField] = xgo.Int32ToStr(b.TotalBoard)
	m[PlayerTotalEarnField] = xgo.Float64ToStr(b.TotalEarn)
	m[PlayerTotalConsumeField] = xgo.Float64ToStr(b.TotalConsume)
	m[PlayerAllTotalBoardField] = xgo.Int64ToStr(b.AllTotalBoard)
	return m
}
