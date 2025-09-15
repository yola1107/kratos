package calc

// Compare 比牌 a > b 返回true 否则返回false
func Compare(a, b HandCard) bool {
	if a.Type < b.Type {
		return true
	}

	// 牌型相同对比牌值，牌值都相同，就比较第一个的花牌
	if a.Type == b.Type {
		for i := 0; i < 3; i++ {
			as, bs := GetCardScore(a.Cards[i]), GetCardScore(b.Cards[i])
			if as == bs {
				continue
			}
			if as > bs {
				return true
			}
			return false
		}

		if GetCardColor(a.Cards[0]) > GetCardColor(b.Cards[0]) {
			return true
		}
	}

	return false
}

// DebugCard 配牌
func DebugCard(deck []int, combinationArray []CardType) []int {
	if len(deck) == 0 {
		return deck
	}

	// 获取所有可能的整数形式
	result := findCombinations(intToCards(deck), combinationArray)

	// 获取所有整形组合中的牌
	genCards := []int(nil)
	for _, v := range result {
		genCards = append(genCards, v.cards...)
	}

	// 生成新的卡牌堆 调整堆中的卡牌顺序
	heap := make([]int, len(deck))
	copy(heap, deck)
	for idx := 0; idx < len(genCards); idx++ {
		if heap[idx] != genCards[idx] {
			for j := idx + 1; j < len(heap); j++ {
				if heap[j] == genCards[idx] {
					heap[idx], heap[j] = heap[j], heap[idx]
					break
				}
			}
		}
	}

	// 返回调整后的牌堆
	return heap
}
