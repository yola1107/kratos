package loop

type ILoop interface {
	Start()
	Stop()
	Jobs() int
	Post(job func())
	PostAndWait(job func() any) any
}
type taskBuffer struct {
	jobs   chan func()
	toggle chan byte
}
