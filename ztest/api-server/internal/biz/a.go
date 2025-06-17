package biz

/*

entity 想通知 usecase 做事	定义回调接口并注入
entity 引用 usecase 导致循环引用	反转依赖：usecase 实现接口
usecase 调用 entity	直接调用方法（无问题） 正常向下依赖


internal/
├── biz/                 # usecase 逻辑
│   └── game.go
├── conf/                # 配置定义（conf.proto）
├── data/                # repo 实现
│   ├── data.go
│   └── gamerepo.go
├── entity/              # 领域模型层
│   ├── table/           # 桌子核心逻辑（Table, State, Timer 等）
│   └── playercore/      # 玩家核心逻辑（Player, Hand, Actions 等）
├── game/                # 对外组合：管理 TableMgr / PlayerMgr 等
│   ├── playermgr.go
│   ├── tablemgr.go
│   └── provider.go      # wire 构造
├── service/             # protobuf 服务层
│   └── game.go
├── server/              # 启动服务（grpc/ws/http）
│   └── server.go
├── di/                  # wire 注入层
│   ├── wire.go
│   └── wire_gen.go
main.go

整体架构分层图（基于 DDD + Kratos）
        +-------------------+
        |     Client        |
        | (WebSocket/HTTP)  |
        +--------+----------+
                 ↓
        +--------v----------+          <- interface/websocket/
        |  Transport Layer  |          <- 接入层（协议、连接、消息解码）
        +--------+----------+
                 ↓
        +--------v----------+          <- service/
        |   Service Layer   |          <- 协调层，封装 UseCase 调用
        +--------+----------+
                 ↓
        +--------v----------+          <- biz/usecase/
        |   UseCase Layer   |          <- 游戏行为（开始游戏、出牌等）
        +--------+----------+
                 ↓
        +--------v----------+          <- biz/entity/ & manager/
        |   Domain Layer    |          <- 实体（Player/Table）和管理器
        +--------+----------+
                 ↓
        +--------v----------+          <- data/
        |   Persistence     |          <- Redis、DB、内存、文件等
        +-------------------+



组件依赖关系图（构造顺序）

conf.Game
  ↓
NewTableMgr(conf.Game, logger)
  ↓
NewPlayerMgr(logger)
  ↓
NewGameUsecase(TableMgr, PlayerMgr, GameRepo)
  ↓
NewGameService(GameUsecase)
  ↓
NewGRPCServer(GameService)
  ↓
newApp(grpc.Server)

*/
