package proto

const (
	// OpProtoReady proto ready
	OpProtoReady = int32(1)
	// OpProtoFinish proto finish
	OpProtoFinish = int32(2)
)

const (
	OpPush     = int32(0)
	OpPing     = int32(1)
	OpPong     = int32(2)
	OpRequest  = int32(3)
	OpResponse = int32(4)
	OpSub      = int32(5)
	OpUnsub    = int32(6)
	OpPub      = int32(7)
)

const (
	AuthOps = -1
)

const (
	PlaceClient = 0
	PlaceServer = 1
)
