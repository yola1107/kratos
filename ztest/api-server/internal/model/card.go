package model

import (
	"log"

	"github.com/yola1107/kratos/v2/library/ext"
)

type GameCards struct {
	cardHeap []int // 所有牌
	index    int   // 牌型使用位置
}

func (g *GameCards) Init() {
	g.cardHeap = make([]int, 0, 13*4)
	for i := 0; i < 4; i++ {
		for j := 1; j < 14; j++ {
			g.cardHeap = append(g.cardHeap, j+i*0x10)
		}
	}

	g.index = 0
}

func (g *GameCards) Shuffle() {
	for i := 0; i < 3; i++ {
		ext.SliceShuffle(g.cardHeap)
	}
}

func (g *GameCards) DispatchCards(n int) []int {
	if g.index+n >= len(g.cardHeap) {
		log.Printf("GameCards error.(overflow) cidx:%d n:%d", g.index, n)
		return make([]int, n)
	}

	g.index += n

	return g.cardHeap[g.index-n : g.index]
}
