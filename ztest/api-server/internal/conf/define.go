package conf

import (
	"flag"
	"os"
)

const GameID = 130
const Name = "api-server"
const Version = "v0.0.1"

var (
	ArenaID  = 1  //场ID: 1 2 3 4
	ServerID = "" //房间ID
)

func init() {
	flag.IntVar(&ArenaID, "aid", 1, "specify the arena ID. base.StrToInt(os.Getenv(\"ARENAID\"))")
	flag.StringVar(&ServerID, "sid", os.Getenv("HOSTNAME"), "specify the server ID.")
}
