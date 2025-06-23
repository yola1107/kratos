package table

import (
	"github.com/yola1107/kratos/v2/library/ext"
	"github.com/yola1107/kratos/v2/log"
)

type GameCards struct {
	cardHeap []int32 // 所有牌
	index    int     // 牌型使用位置
}

func NewGameCards() *GameCards {
	g := &GameCards{}
	g.Init()
	return g
}

func (g *GameCards) Init() {
	g.index = 0
	g.cardHeap = make([]int32, 0, 13*4)
	for i := 0; i < 4; i++ {
		for j := 1; j < 14; j++ {
			g.cardHeap = append(g.cardHeap, int32(j+i*0x10))
		}
	}
}

func (g *GameCards) Shuffle() {
	if len(g.cardHeap) != 13*4 {
		g.Init() // 容错处理
		log.Errorf("WARNING: Shuffle called with unexpected length:%d cidx:%d", len(g.cardHeap), g.index)
	}

	g.index = 0
	for i := 0; i < 3; i++ {
		ext.SliceShuffle(g.cardHeap)
	}
}

func (g *GameCards) DispatchCards(n int) []int32 {
	if g.index+n > len(g.cardHeap) {
		log.Errorf("GameCards error.(overflow) cidx:%d n:%d total=%d", g.index, n, len(g.cardHeap))
		return nil // make([]int32, n)
	}

	start := g.index
	g.index += n
	return ext.SliceCopy(g.cardHeap[start:g.index])
}
