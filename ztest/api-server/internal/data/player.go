package data

import (
	"context"
	"fmt"

	"github.com/yola1107/kratos/v2/errors"
	"github.com/yola1107/kratos/v2/library/ext"
	"github.com/yola1107/kratos/v2/ztest/api-server/internal/biz/player"
	. "github.com/yola1107/kratos/v2/ztest/api-server/pkg/xredis"
)

var (
	errRedisNil = errors.New(1, "", "no exist")
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
	values, err := r.data.redis.HMGet(ctx, key, allBaseDataFields...).Result()
	if err != nil {
		return nil, errors.New(1, "", err.Error())
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

	b.UID = ext.StrToInt64(data[PlayerUIDField])
	b.VIP = ext.StrToInt32(data[PlayerVIPField])
	b.NickName = data[PlayerNickNameField]
	b.Avatar = data[PlayerAvatarField]
	b.AvatarUrl = data[PlayerAvatarUrlField]
	b.Money = ext.StrToFloat64(data[PlayerMoneyField])
	b.ChannelID = ext.StrToInt32(data[PlayerChannelIDField])
	b.PayTotal = ext.StrToFloat64(data[PlayerPayTotalField])
	b.WithdrawTotal = ext.StrToFloat64(data[PlayerWithdrawTotalField])
	b.PresentBet = ext.StrToFloat64(data[PlayerPresentBetField])
	b.PresentProfit = ext.StrToFloat64(data[PlayerPresentProfitField])
	b.PresentWinScore = ext.StrToFloat64(data[PlayerPresentWinScoreField])
	b.PresentBoard = ext.StrToInt32(data[PlayerPresentBoardField])
	b.TotalBoard = ext.StrToInt32(data[PlayerTotalBoardField])
	b.TotalEarn = ext.StrToFloat64(data[PlayerTotalEarnField])
	b.TotalConsume = ext.StrToFloat64(data[PlayerTotalConsumeField])
	b.AllTotalBoard = ext.StrToInt64(data[PlayerAllTotalBoardField])

	b.UID = uid
	return b
}

// ToRedisMap 转为 Redis hash 的 map[string]string
func ToRedisMap(b *player.BaseData) map[string]string {
	m := make(map[string]string)
	m[PlayerUIDField] = ext.Int64ToStr(b.UID)
	m[PlayerMoneyField] = ext.Float64ToStr(b.Money)
	m[PlayerChannelIDField] = ext.Int32ToStr(b.ChannelID)
	m[PlayerPayTotalField] = ext.Float64ToStr(b.PayTotal)
	m[PlayerWithdrawTotalField] = ext.Float64ToStr(b.WithdrawTotal)
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
