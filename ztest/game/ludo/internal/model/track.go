package model

import (
	"fmt"
	"math"
	"time"

	"github.com/yola1107/kratos/v2/log"
)

/*
	棋盘棋子可移动路径的全排列. m颗棋子n颗骰子
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

// Permute 递归回溯.  m颗棋子n颗骰子的全部路径信息
func Permute(b *Board, color int32, _dices []int32, all bool) (ret *TagRetData) {
	if b == nil || len(_dices) == 0 {
		return nil
	}

	dices := append([]int32(nil), _dices...) // 拷贝骰子，防止修改外部数据
	ret = &TagRetData{}

	// 若无可移动棋子直接返回
	if ids := b.GetActivePieceIDs(color); len(ids) == 0 {
		return
	}

	// 计算可移动棋子全部有效路径
	start := time.Now()
	visited := make([]bool, len(dices))
	b.permute(color, []int32(nil), ret, dices, visited, len(dices), all, 0)

	if false {
		fmt.Printf("Permute. use:%v/ms color:%+v dices:%+v ret:%+v\n", time.Since(start).Milliseconds(), color, dices, ret.Desc())
	}
	return
}

func (b *Board) permute(color int32, cache []int32, _Ret *TagRetData, dices []int32, visited []bool, max int, all bool, index int) {
	// 剪枝：找到最长路径即可. 找到一组 退出
	if (!all) && _Ret.Max == max {
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

		for _, v := range b.GetActivePieceIDs(color) {
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
		for i := 0; i < len(moves); i += 2 {
			step := c.moveOne(moves[i], moves[i+1])
			steps = append(steps, step)
		}
		if score := c.evaluateMoveSequence(steps, totalDiceSum, color, _defaultParameter); score > bestScore {
			bestScore = score
			bestPath = append([]int32(nil), moves...) // copy
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

type parameter struct {
	threatenedDis int32 // 威胁 表示“我能追到对方”的阈值
	dangerousDis  int32 // 危险 表示“敌人能追到我”的阈值
}

var _defaultParameter = parameter{
	threatenedDis: 6,
	dangerousDis:  6,
}

// evaluateMoveSequence 评估移动序列的得分函数，考虑击杀数、移动距离、特殊奖励和惩罚
func (b *Board) evaluateMoveSequence(steps []*Step, totalDiceSum, color int32, pa parameter) int32 {
	score := int32(0)
	usedDiceSum := int32(0)

	for i, step := range steps {
		if step.From == step.To {
			continue
		}
		moved := step.X
		score += moved * 2 // 基础移动奖励
		usedDiceSum += moved

		// 击杀奖励
		for _, killed := range step.Killed {
			score += StepsFromStart(killed.From, killed.Color)*2 + int32(len(steps)-i)*2 + 20
		}

		// 出基地奖励
		if step.From == BasePos {
			score += 60
		}

		// 到达终点奖励
		if mover := b.GetPieceByID(step.Id); mover != nil && mover.IsArrived() {
			score += 80
		}
	}

	// 危险/威胁
	pieces, enemy := spitEnemyPieces(b, color)
	for _, p := range pieces {
		if _, safe := SafePositions[p.pos]; safe {
			score += 15 // 占安全点轻微加分
		}
		if p.state == PieceInHomePath {
			score += 2 // 家路径轻微加分
		}

		if p.IsOnBoard() {
			// 越靠近终点，奖励越高（尽量推进）
			stepsToEnd := (p.pos - HomeEntrances[p.color] + TotalPositions) % TotalPositions // distForward(p.pos, HomeEntrances[p.color], TotalPositions)
			score += (TotalPositions - stepsToEnd) / 5

			// 危险/威胁
			for _, e := range enemy {
				if !e.IsOnBoard() || !p.IsEnemy(e) {
					continue
				}

				dis := p.pos - e.pos

				// 危险：敌人在我后面，可能追上来
				if dis > 0 && dis <= pa.dangerousDis {
					score -= (pa.dangerousDis - dis + 1) * 2 // 越近惩罚越大
				}

				// 威胁：我在敌人后面，可以追杀
				if dis < 0 && -dis <= pa.threatenedDis {
					score += (pa.threatenedDis + dis + 1) / 2 // 越近奖励越大
				}
			}
		}
	}

	// 奖励用完点数
	score += usedDiceSum

	// 惩罚浪费
	if wasted := totalDiceSum - usedDiceSum; wasted > 0 {
		score -= wasted
	}

	return score
}

// 获取己方和敌方可活动棋子（未到终点）
func spitEnemyPieces(b *Board, color int32) ([]*Piece, []*Piece) {
	var pieces, enemy []*Piece
	for _, p := range b.Pieces() {
		if p == nil || p.IsArrived() {
			continue
		}
		if p.color == color {
			pieces = append(pieces, p)
		} else {
			enemy = append(enemy, p)
		}
	}
	return pieces, enemy
}
