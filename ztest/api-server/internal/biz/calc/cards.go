package calc

import (
	"fmt"
	"sort"

	"github.com/yola1107/kratos/v2/log"
)

// CardType 牌型
type CardType int32

const (
	CtInvalid CardType = iota // 错误牌型
	CtBaoZi                   // 豹子
	CtShunJin                 // 顺金
	CtShunZi                  // 顺子
	CtJinHua                  // 金花
	CtDuiZi                   // 对子
	CtDanPai                  // 单牌
)

func GetCardColor(card int32) int32 { return card / 0x10 }
func GetCardNum(card int32) int32   { return card % 0x10 }
func GetCardScore(card int32) int32 {
	num := GetCardNum(card)
	if num == 1 {
		return 14 // A > K
	}
	return num
}

/*
	HandCard 手牌

*/

type HandCard struct {
	Cards []int32
	Type  CardType
}

func (h *HandCard) Set(cards []int32) {
	if len(cards) != 3 {
		log.Errorf("Set cards error, cards length is not 3: %d", len(cards))
		return
	}
	h.Cards = make([]int32, 3)
	copy(h.Cards, cards)
	h.CalcType()
}

func (h *HandCard) CalcType() {
	if len(h.Cards) != 3 {
		log.Errorf("Set cards error, cards length is not 3: %v", h.Cards)
		return
	}

	// 拷贝并排序：从大到小
	sort.Slice(h.Cards, func(i, j int) bool {
		return GetCardScore(h.Cards[i]) > GetCardScore(h.Cards[j])
	})
	card0, card1, card2 := h.Cards[0], h.Cards[1], h.Cards[2]
	num0, num1, num2 := GetCardNum(card0), GetCardNum(card1), GetCardNum(card2)
	score0, score1, score2 := GetCardScore(card0), GetCardScore(card1), GetCardScore(card2)
	color0, color1, color2 := GetCardColor(card0), GetCardColor(card1), GetCardColor(card2)

	// 豹子
	if num0 == num1 && num1 == num2 {
		h.Type = CtBaoZi
		return
	}

	// 顺子
	isSequence := score0 == score1+1 && score1 == score2+1
	if !isSequence && score0 == 14 {
		// A23 特殊顺子
		isSequence = num1 == 3 && num2 == 2
	}

	// 同花
	isSameColor := color0 == color1 && color1 == color2

	switch {
	case isSequence && isSameColor:
		h.Type = CtShunJin
	case isSequence:
		h.Type = CtShunZi
	case isSameColor:
		h.Type = CtJinHua
	case num0 == num1 || num1 == num2:
		h.Type = CtDuiZi
		// 把对子牌移到前面
		if num0 != num1 {
			h.Cards[0], h.Cards[2] = h.Cards[2], h.Cards[0]
		}
	default:
		h.Type = CtDanPai
	}
}

func (h *HandCard) String() string {
	return fmt.Sprintf("(%v %v)", h.Cards, h.Type)
}
