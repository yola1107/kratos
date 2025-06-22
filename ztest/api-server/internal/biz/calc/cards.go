package calc

import (
	"fmt"
	"sort"
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

// GetCardColor 获取牌花色
func GetCardColor(card int32) int32 {
	return card / 0x10
}

// GetCardNum 获取牌值
func GetCardNum(card int32) int32 {
	return card % 0x10
}

// GetCardScore 获取牌的大小值
func GetCardScore(card int32) int32 {
	num := GetCardNum(card)
	// A>K
	if num == 1 {
		num = 14
	}
	return num
}

/*
	HandCard 手牌

*/

type HandCard struct {
	Cards []int32  // 手牌
	Type  CardType // 牌型
}

// Set 设置手牌
func (h *HandCard) Set(cards []int32) {
	h.Cards = make([]int32, len(cards))
	copy(h.Cards, cards)
	h.CalcType()
}

// CalcType 计算牌型
func (h *HandCard) CalcType() {
	if len(h.Cards) != 3 {
		return
	}

	sort.Slice(h.Cards, func(i, j int) bool {
		return GetCardScore(h.Cards[i]) > GetCardScore(h.Cards[j])
	})

	// 炸弹
	if GetCardNum(h.Cards[0]) == GetCardNum(h.Cards[1]) &&
		GetCardNum(h.Cards[1]) == GetCardNum(h.Cards[2]) {
		h.Type = CtBaoZi
		return
	}

	isSequence := GetCardScore(h.Cards[0]) == GetCardScore(h.Cards[1])+1 &&
		GetCardScore(h.Cards[1]) == GetCardScore(h.Cards[2])+1

	if !isSequence && GetCardScore(h.Cards[0]) == 14 {
		isSequence = GetCardNum(h.Cards[1]) == GetCardNum(h.Cards[2])+1 &&
			GetCardNum(h.Cards[0])+1 == GetCardNum(h.Cards[2])
	}
	isColour := GetCardColor(h.Cards[0]) == GetCardColor(h.Cards[1]) &&
		GetCardColor(h.Cards[1]) == GetCardColor(h.Cards[2])

	switch {
	case isSequence && isColour:
		h.Type = CtShunJin
	case isSequence:
		h.Type = CtShunZi
	case isColour:
		h.Type = CtJinHua
	default:
		if GetCardNum(h.Cards[0]) == GetCardNum(h.Cards[1]) ||
			GetCardNum(h.Cards[1]) == GetCardNum(h.Cards[2]) {
			h.Type = CtDuiZi
			// 对子放前面
			if GetCardNum(h.Cards[0]) != GetCardNum(h.Cards[1]) {
				h.Cards[0], h.Cards[2] = h.Cards[2], h.Cards[0]
			}
		} else {
			h.Type = CtDanPai
		}
	}

}

// ToString 用于记录
func (h *HandCard) String() string {
	return fmt.Sprintf("(%v %v)", h.Cards, h.Type)
}
