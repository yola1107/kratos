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
	ids := b.GetActivePieceIDs(color)
	if len(ids) == 0 {
		return
	}

	// 计算可移动棋子全部有效路径
	start := time.Now()
	visitedMask := int32(0) // 位掩码优化
	b.permute(color, []int32(nil), ret, dices, visitedMask, len(dices), all, ids)

	if false {
		fmt.Printf("Permute. use:%v/ms color:%+v dices:%+v ret:%+v\n", time.Since(start).Milliseconds(), color, dices, ret.Desc())
	}
	return
}

func (b *Board) permute(color int32, cache []int32, _Ret *TagRetData, dices []int32, visitedMask int32, max int, all bool, ids []int32) {
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
		if visitedMask&(1<<j) != 0 { // 如果骰子已使用，跳过
			continue
		}

		for _, v := range ids {
			moverID := v
			if ok, _ := b.canMoveOne(moverID, dices[j]); !ok {
				continue
			}

			// 标记该骰子已使用
			visitedMask |= (1 << j)
			cache = append(cache, moverID, dices[j])
			b.moveOne(moverID, dices[j])
			b.permute(color, cache, _Ret, dices, visitedMask, max, all, ids)
			b.backOne()
			cache = cache[:len(cache)-2]
			visitedMask &= ^(1 << j) // 取消标记已用骰子
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

//
// package model
//
// import (
// 	"math"
// )
//
// var (
// 	_dangerousDis  = int32(6)
// 	_threatenedDis = int32(6)
// )
//
// // 优化后的递归评分数据
// type bestMoveData struct {
// 	bestScore int32
// 	bestPath  []int32
// }
//
// /*
// 	FindBestMoveSequence 返回最优移动序列及其得分。
// 		例如有 dices=[2,6], 而刚好2+6=8可以kill对方棋子.
// */
//
// // FindBestMoveSequence 返回最佳移动序列第一步，内存优化版
// func FindBestMoveSequence(c *Board, dices []int32, color int32) (int32, int32) {
// 	if c == nil || len(dices) == 0 {
// 		return -1, -1
// 	}
//
// 	data := &bestMoveData{
// 		bestScore: math.MinInt32,
// 		bestPath:  nil,
// 	}
//
// 	cache := make([]int32, 0, len(dices)*2) // 当前路径缓存
// 	visited := make([]bool, len(dices))     // 递归标记骰子是否使用
// 	dfsEvaluate(c, color, dices, visited, cache, data)
//
// 	if len(data.bestPath) > 1 {
// 		return data.bestPath[0], data.bestPath[1]
// 	}
// 	return -1, -1
// }
//
// // dfsEvaluate 遍历所有移动序列，同时计算分数，避免生成所有路径
// func dfsEvaluate(b *Board, color int32, dices []int32, visited []bool, cache []int32, data *bestMoveData) {
// 	// 如果当前路径长度达到缓存长度，尝试计算分数
// 	if len(cache)/2 == len(dices) {
// 		steps := make([]*Step, 0, len(dices))
// 		for i := 0; i < len(cache); i += 2 {
// 			step := b.moveOne(cache[i], cache[i+1])
// 			steps = append(steps, step)
// 		}
//
// 		score := b.evaluateMoveSequence(steps, sumDices(dices), color)
// 		if score > data.bestScore {
// 			data.bestScore = score
// 			data.bestPath = append([]int32(nil), cache...)
// 		}
//
// 		for range steps {
// 			b.backOne()
// 		}
// 		return
// 	}
//
// 	for j := 0; j < len(dices); j++ {
// 		if visited[j] {
// 			continue
// 		}
//
// 		for _, pid := range b.GetActivePieceIDs(color) {
// 			if ok, _ := b.canMoveOne(pid, dices[j]); !ok {
// 				continue
// 			}
//
// 			visited[j] = true
// 			cache = append(cache, pid, dices[j])
// 			b.moveOne(pid, dices[j])
//
// 			dfsEvaluate(b, color, dices, visited, cache, data)
//
// 			b.backOne()
// 			cache = cache[:len(cache)-2]
// 			visited[j] = false
// 		}
// 	}
// }
//
// // sumDices 计算骰子总和
// func sumDices(dices []int32) int32 {
// 	var total int32
// 	for _, d := range dices {
// 		total += d
// 	}
// 	return total
// }
//
// // evaluateMoveSequence 评估移动序列的得分函数，考虑击杀数、移动距离、特殊奖励和惩罚
// func (b *Board) evaluateMoveSequence(steps []*Step, totalDiceSum, color int32) int32 {
// 	score := int32(0)
// 	usedDiceSum := int32(0)
//
// 	for i, step := range steps {
// 		if step.From == step.To {
// 			continue
// 		}
// 		moved := step.X
// 		score += moved * 2 // 基础移动奖励
// 		usedDiceSum += moved
//
// 		// 击杀奖励
// 		for _, killed := range step.Killed {
// 			score += StepsFromStart(killed.From, killed.Color)*2 + int32(len(steps)-i)*2 + 20
// 		}
//
// 		// 出基地奖励
// 		if step.From == BasePos {
// 			score += 60
// 		}
//
// 		// 到达终点奖励
// 		if mover := b.GetPieceByID(step.Id); mover != nil && mover.IsArrived() {
// 			score += 80
// 		}
// 	}
//
// 	// 危险/威胁
// 	pieces, enemy := spitEnemyPieces(b, color)
// 	for _, p := range pieces {
// 		if _, safe := SafePositions[p.pos]; safe {
// 			score += 15 // 占安全点轻微加分
// 		}
// 		if p.state == PieceInHomePath {
// 			score += 2 // 家路径轻微加分
// 		}
//
// 		if p.IsOnBoard() {
// 			// 越靠近终点，奖励越高（尽量推进）
// 			stepsToEnd := (p.pos - HomeEntrances[p.color] + TotalPositions) % TotalPositions // distForward(p.pos, HomeEntrances[p.color], TotalPositions)
// 			score += (TotalPositions - stepsToEnd) / 5
//
// 			// 危险/威胁
// 			for _, e := range enemy {
// 				if !e.IsOnBoard() || !p.IsEnemy(e) {
// 					continue
// 				}
//
// 				dis := p.pos - e.pos
//
// 				// 危险：敌人在我后面，可能追上来
// 				if dis > 0 && dis <= _dangerousDis {
// 					score -= (_dangerousDis - dis + 1) * 2 // 越近惩罚越大
// 				}
//
// 				// 威胁：我在敌人后面，可以追杀
// 				if dis < 0 && -dis <= _threatenedDis {
// 					score += (_threatenedDis + dis + 1) / 2 // 越近奖励越大
// 				}
// 			}
// 		}
// 	}
//
// 	// 奖励用完点数
// 	score += usedDiceSum
//
// 	// 惩罚浪费
// 	if wasted := totalDiceSum - usedDiceSum; wasted > 0 {
// 		score -= wasted
// 	}
//
// 	return score
// }
//
// // 获取己方和敌方可活动棋子（未到终点）
// func spitEnemyPieces(b *Board, color int32) ([]*Piece, []*Piece) {
// 	var pieces, enemy []*Piece
// 	for _, p := range b.Pieces() {
// 		if p == nil || p.IsArrived() {
// 			continue
// 		}
// 		if p.color == color {
// 			pieces = append(pieces, p)
// 		} else {
// 			enemy = append(enemy, p)
// 		}
// 	}
// 	return pieces, enemy
// }
