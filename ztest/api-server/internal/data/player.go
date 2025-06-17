package data

import (
	"context"
	"fmt"

	"github.com/yola1107/kratos/v2/ztest/api-server/internal/biz/player"
)

func (r *dataRepo) Save(ctx context.Context, p *player.BaseData) error {
	// 将 player.BaseData 转成 DO（数据库模型），写入数据库
	fmt.Printf("保存玩家 %d 到数据库: %+v\n", p.UID, p.NickName)
	return nil
}

func (r *dataRepo) Load(ctx context.Context, playerID int64) (*player.BaseData, error) {
	// 从数据库读取并还原为 player.BaseData
	fmt.Printf("从数据库加载玩家 %d\n", playerID)
	return &player.BaseData{
		UID:      playerID,
		NickName: "测试玩家",
	}, nil
}
