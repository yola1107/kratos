package conf

import (
	"flag"
	"os"
)

const Name = "api-server"
const Version = "v0.0.1"
const GameID = 130

var ArenaID = 1   //场ID: 1 2 3 4
var ServerID = "" //房间ID

func init() {
	flag.IntVar(&ArenaID, "aid", 1, "specify the arena ID. base.StrToInt(os.Getenv(\"ARENAID\"))")
	flag.StringVar(&ServerID, "sid", os.Getenv("HOSTNAME"), "specify the server ID.")
}

// 阶段定义
const (
	StPrepare  = 0  //准备期/空闲期
	StSendCard = 1  //发牌期
	StGetCard  = 2  //等待抓牌
	StPlayCard = 3  //等待出牌
	StDismiss  = 4  //解散状态
	StSmallEnd = 5  //小局结束
	StResult   = 10 //结算
)
