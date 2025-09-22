package model

import "math"

var (
	_threatenedDis = int32(6)
	_dangerousDis  = int32(6)
)

type bestMoveData struct {
	bestScore int32
	bestPath  []int32
	maxStep   int // 记录最佳序列的步数
}

// FindBestMoveSequence 返回最佳移动序列第一步
func FindBestMoveSequence(b *Board, dices []int32, color int32) (int32, int32) {
	if b == nil || len(dices) == 0 {
		return -1, -1
	}

	c := b.Clone()
	defer func() { c.Clear(); c = nil }()

	var ids, enemy []int32
	for _, p := range c.Pieces() {
		if p == nil || p.IsArrived() {
			continue
		}
		if p.color == color {
			ids = append(ids, p.id)
		} else {
			enemy = append(enemy, p.id)
		}
	}
	if len(ids) == 0 {
		return -1, -1
	}

	data := &bestMoveData{
		bestScore: math.MinInt32,
		bestPath:  nil,
	}

	cache := make([]int32, 0, len(dices)*2)  // 当前路径缓存
	visited := make([]bool, len(dices))      // 标记骰子是否使用
	stepsBuf := make([]*Step, 0, len(dices)) // 复用 steps slice
	totalDiceSum := int32(0)
	for _, d := range dices {
		totalDiceSum += d
	}

	dfsEvaluate(c, color, dices, visited, len(dices), cache, stepsBuf, data, ids, enemy, totalDiceSum)

	if len(data.bestPath) > 1 {
		return data.bestPath[0], data.bestPath[1]
	}
	return -1, -1
}

// dfsEvaluate 遍历所有移动序列，同时计算分数，避免生成所有路径
func dfsEvaluate(b *Board, color int32, dices []int32, visited []bool, remainingDice int,
	cache []int32, steps []*Step, data *bestMoveData, ids, enemy []int32, totalDiceSum int32) {

	// 剪枝 1：当前路径 + 剩余骰子 < 当前最大步数，直接回溯
	if len(steps)+remainingDice < data.maxStep {
		return
	}

	// 剪枝 2：剩余骰子无法移动任何棋子，直接回溯
	canMoveAny := false
	for j, d := range dices {
		if visited[j] {
			continue
		}
		for _, pid := range ids {
			if ok, _ := b.canMoveOne(pid, d); ok {
				canMoveAny = true
				break
			}
		}
		if canMoveAny {
			break
		}
	}
	if !canMoveAny {
		return
	}

	// DFS 递归传下来的 steps 就可以直接评估
	if len(steps) > 0 {
		score := b.evaluateMoveSequence(steps, totalDiceSum, color, ids, enemy)
		// 1>优先选择步数最多的路径（用完更多骰子）
		// 2>步数相同的情况下选择分数最高的路径
		if len(steps) > data.maxStep ||
			(score > data.bestScore && len(steps) == data.maxStep) {
			data.maxStep = len(steps)
			data.bestScore = score
			data.bestPath = append(data.bestPath[:0], cache...)
		}
	}

	// 遍历每个骰子和棋子
	for j := 0; j < len(dices); j++ {
		if visited[j] {
			continue
		}

		for _, pid := range ids {
			if ok, _ := b.canMoveOne(pid, dices[j]); !ok {
				continue
			}

			// 标记使用
			visited[j] = true
			cache = append(cache, pid, dices[j])

			// 执行动作并加入 steps
			step := b.moveOne(pid, dices[j])
			steps = append(steps, step)

			// 递归下一层，remainingDice-1
			dfsEvaluate(b, color, dices, visited, remainingDice-1, cache, steps, data, ids, enemy, totalDiceSum)

			// 回溯
			b.backOne()
			cache = cache[:len(cache)-2]
			steps = steps[:len(steps)-1]
			visited[j] = false
		}
	}
}

// evaluateMoveSequence 评估移动序列的得分函数，考虑击杀数、移动距离、特殊奖励和惩罚
func (b *Board) evaluateMoveSequence(steps []*Step, totalDiceSum, color int32, ids, enemy []int32) int32 {
	score := int32(0)
	usedDiceSum := int32(0)

	for i, step := range steps {
		if step.From == step.To {
			continue
		}
		score += step.X * 2 // 基础移动奖励
		usedDiceSum += step.X

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
	for _, id := range ids {
		p := b.GetPieceByID(id)
		if p == nil {
			continue
		}
		if _, safe := SafePositions[p.pos]; safe {
			score += 15 // 占安全点轻微加分
		}
		if p.state == PieceInHomePath {
			score += 2 // 家路径轻微加分
		}

		if p.IsOnBoard() {
			// 越靠近终点，奖励越高（尽量推进）
			stepsToEnd := (p.pos - HomeEntrances[p.color] + TotalPositions) % TotalPositions
			score += (TotalPositions - stepsToEnd) / 5

			// 危险/威胁
			for _, eID := range enemy {
				e := b.GetPieceByID(eID)
				if !e.IsOnBoard() || !p.IsEnemy(e) {
					continue
				}

				forwardDist := (e.pos - p.pos + TotalPositions) % TotalPositions  // p到e的顺时针距离
				backwardDist := (p.pos - e.pos + TotalPositions) % TotalPositions // e到p的顺时针距离

				// 危险：敌人在我后面，可能追上来
				if backwardDist > 0 && backwardDist <= _dangerousDis {
					score -= (_dangerousDis - backwardDist + 1) * 2 // 越近惩罚越大
				}

				// 威胁：我在敌人后面，可以追杀
				if forwardDist > 0 && forwardDist <= _threatenedDis {
					score += (_threatenedDis - forwardDist + 1) / 2 // 越近奖励越大
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
