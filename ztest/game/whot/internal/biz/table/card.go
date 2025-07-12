package table

import (
	"github.com/yola1107/kratos/v2/library/ext"
)

func init() {
	for _, v := range oneDeck {
		deckMap[v] = struct{}{}
	}
}

const (
	SuitMask = 100
	WhotCard = 620
)

var deckMap = map[int32]struct{}{}

// 生成一副牌 54张
var oneDeck = []int32{
	101, 102, 103, 104, 105, 107, 108, 110, 111, 112, 113, 114, // Circle 12
	201, 202, 203, 204, 205, 207, 208, 210, 211, 212, 213, 214, // Triangle 12
	301, 302, 303, 305, 307, 310, 311, 313, 314, // Cross 9
	401, 402, 403, 405, 407, 410, 411, 413, 414, // quare 9
	501, 502, 503, 504, 505, 507, 508, // Star 7
	WhotCard, WhotCard, WhotCard, WhotCard, WhotCard, // Whot 5
}

// NewCard 创建牌，编码格式：花色*100 + 点数（WHOT点数为20）
func NewCard(suit, number int32) int32 {
	return suit*SuitMask + number
}

// NewDeclareWhot 创建牌declareSuit whotCard修改suit
func NewDeclareWhot(suit int32, card int32) int32 {
	return suit*SuitMask + Number(card)
}

// Suit 返回花色
func Suit(card int32) int32 {
	return card / SuitMask
}

// Number 返回点数
func Number(card int32) int32 {
	return card % SuitMask
}

func IsValidCard(card int32) bool {
	_, ok := deckMap[card]
	return ok
}

// ValidBottom 合法的底牌 非功能牌
func ValidBottom(card int32) bool {
	return !IsSpecialCard(card)
}

// IsSpecialCard 功能牌说明：其中1，2，8，14，20为功能牌
func IsSpecialCard(card int32) bool {
	number := Number(card)
	return number == 1 || number == 2 || number == 8 || number == 14 || number == 20
}

// GameCards 管理牌堆
type GameCards struct {
	index  int
	cards  []int32
	bottom []int32
}

// NewGameCards 初始化牌堆
func NewGameCards() *GameCards {
	cards := make([]int32, len(oneDeck))
	copy(cards, oneDeck)
	return &GameCards{cards: cards}
}

// Shuffle 洗牌并重置索引
func (g *GameCards) Shuffle() {
	g.index = 0
	for i := 0; i < 3; i++ {
		ext.SliceShuffle(g.cards)
	}
}

// DispatchCards 发牌，返回 n 张牌
func (g *GameCards) DispatchCards(n int) []int32 {
	end := g.index + n
	if end > len(g.cards) {
		end = len(g.cards)
	}
	cards := ext.SliceCopy(g.cards[g.index:end])
	g.index = end
	return cards
}

// GetCardNum 返回剩余牌数
func (g *GameCards) GetCardNum() int32 {
	return int32(len(g.cards) - g.index)
}

// IsEmpty 是否牌堆空了
func (g *GameCards) IsEmpty() bool {
	return g.index >= len(g.cards)
}

// SetBottom 设置底牌
func (g *GameCards) SetBottom() []int32 {
	if !ValidBottom(g.cards[g.index]) {
		for i := g.index + 1; i < len(g.cards); i++ {
			if !ValidBottom(g.cards[i]) {
				continue
			}
			g.cards[g.index], g.cards[i] = g.cards[i], g.cards[g.index]
			break
		}
	}
	g.bottom = g.DispatchCards(1)
	return g.bottom
}
