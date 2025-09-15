package model

import (
	"fmt"
	"math"
	"time"

	"github.com/yola1107/kratos/v2/log"
)

/*
	棋盘棋子可移动路径的全排列. m颗棋子n颗色子
	递归回溯
*/

// TagRetData 遍历结果
type TagRetData struct {
	Max   int       // 最大步数
	Cache []int32   // 路径缓存
	Dst   [][]int32 // 所有可行路径
}

func (r *TagRetData) Desc() string {
	return fmt.Sprintf("[Max:%+v cache:%+v cnt:%+v]", r.Max, r.Cache, len(r.Dst))
}

func Permute(b *Board, color int32, _dices []int32, all bool) (ret *TagRetData) {
	if b == nil || len(_dices) == 0 {
		return nil
	}

	dices := append([]int32(nil), _dices...) // 拷贝骰子，防止修改外部数据
	ret = &TagRetData{}

	// 若无可移动棋子直接返回
	if ids := b.ActivePieceIDs(color); len(ids) == 0 {
		return
	}

	// 计算可移动棋子全部有效路径
	start := time.Now()
	visited := make([]bool, len(dices))
	b.permute(color, []int32(nil), ret, dices, visited, len(dices), all, 0)

	if false {
		log.Infof("Permute. use:%v/ms color:%+v dices:%+v ret:%+v", time.Since(start).Milliseconds(), color, dices, ret.Desc())
	}
	return
}

func (b *Board) permute(color int32, cache []int32, _Ret *TagRetData, dices []int32, visited []bool, max int, all bool, index int) {
	// 剪枝：找到最长路径即可. 找到一组 退出
	if !all && _Ret.Max == max {
		return
	}

	if len(cache)/2 == _Ret.Max {
		var c = make([]int32, len(cache))
		copy(c, cache)
		_Ret.Dst = append(_Ret.Dst, c)
	}

	if len(cache)/2 > _Ret.Max {
		var c = make([]int32, len(cache))
		copy(c, cache)
		_Ret.Max = len(c) / 2
		_Ret.Cache = c
		_Ret.Dst = [][]int32{c}
	}

	// 遍历每一个骰子点数 + 每一个可移动棋子
	for j := 0; j < len(dices); j++ {
		if visited[j] {
			continue
		}

		for _, v := range b.ActivePieceIDs(color) {
			moverID := v
			if ok, _ := b.canMoveOne(moverID, dices[j]); !ok {
				continue
			}

			visited[j] = true
			cache = append(cache, moverID, dices[j])
			b.moveOne(moverID, dices[j])
			b.permute(color, cache, _Ret, dices, visited, max, all, index+1)
			b.backOne()
			cache = cache[:len(cache)-2]
			visited[j] = false
		}
	}
}

/*
	FindBestMoveSequence 返回最优移动序列及其得分。
		例如有 dices=[2,6], 而刚好2+6=8可以kill对方棋子.
*/

// FindBestMoveSequence 计算较优得分路径
func FindBestMoveSequence(c *Board, dices []int32, color int32) (int32, int32) {
	if c == nil || len(dices) == 0 {
		return -1, -1
	}

	ret := Permute(c, color, dices, true)
	if ret == nil || ret.Max == 0 {
		return -1, -1
	}
	totalDiceSum := int32(0)
	for _, v := range dices {
		if v <= 0 || v > 6 {
			log.Errorf("====== dice invalid. v=%+v", v)
			return -1, -1
		}
		totalDiceSum += v
	}

	bestScore, bestPath := int32(math.MinInt32), []int32(nil)
	for _, moves := range ret.Dst {
		var steps []*Step
		for i := 0; i < len(moves); i = i + 2 {
			step := c.moveOne(moves[i], moves[i+1])
			steps = append(steps, step)
		}
		if score := c.evaluateMoveSequence(steps, totalDiceSum); score > bestScore {
			bestScore = score
			bestPath = moves
		}
		for range steps {
			c.backOne()
		}
	}
	if len(bestPath) > 1 {
		return bestPath[0], bestPath[1] // 返回最佳路径的第一步
	}
	return -1, -1
}

// 评估移动序列的得分函数，考虑击杀数、移动距离、特殊奖励和惩罚
func (b *Board) evaluateMoveSequence(steps []*Step, totalDiceSum int32) int32 {
	score := int32(0)
	usedDiceSum := int32(0)

	for _, step := range steps {
		if step.From == step.To {
			continue // 无实际移动，不参与评分
		}

		moved := step.X
		score += moved * 2
		usedDiceSum += moved

		// 击杀奖励
		score += int32(len(step.Killed)) * 50

		// 出基地奖励
		if step.From == BasePos {
			score += 20
		}

		// 到达终点奖励
		if mover := b.GetPieceByID(step.Id); mover != nil && mover.IsArrived() {
			score += 15
		}
	}

	// 奖励用完点数
	score += usedDiceSum

	// 惩罚浪费
	wasted := totalDiceSum - usedDiceSum
	if wasted > 0 {
		score -= wasted
	}

	return score
}
