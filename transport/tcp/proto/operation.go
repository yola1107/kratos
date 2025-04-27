package proto

const (
	// OpHandshake handshake
	OpHandshake = int32(0)
	// OpHandshakeReply handshake reply
	OpHandshakeReply = int32(1)

	// OpAuth auth connnect
	OpAuth = int32(7)
	// OpAuthReply auth connect reply
	OpAuthReply = int32(8)

	// OpRaw raw message
	OpRaw = int32(9)

	// OpProtoReady proto ready
	OpProtoReady = int32(10)
	// OpProtoFinish proto finish
	OpProtoFinish = int32(11)
)

const (
	PlaceClient = 0
	PlaceServer = 1
)

type Pattern byte

const (
	Push Pattern = iota
	Request
	Response
	Ping
	Pong
	Sub
	Unsub
	Pub
)

const (
	AuthOps = -1
)
