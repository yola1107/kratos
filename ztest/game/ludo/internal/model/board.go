package model

import (
	"github.com/yola1107/kratos/v2/ztest/game/ludo/internal/conf"
)

const (
	MoveOK               int32 = iota // 移动合法，无错误
	ErrInvalidMoveCount               // 移动步数与骰子数量不一致
	ErrInvalidPieceIdx                // 棋子索引无效（ID 不存在或越界）
	ErrInvalidPieceColor              // 棋子颜色与玩家不符
	ErrPieceCannotMove                // 棋子状态不可移动（如已到终点）
	ErrDiceMismatch                   // 移动步数与骰子点数不匹配
	ErrInvalidStep                    // 非法步数（如 <= 0）
	ErrIdleMustBeSix                  // 起始状态只能掷出 6 才能出发
	ErrExceedHomePath                 // 超出终点路径，无法移动
	ErrAlreadyArrived                 // 棋子已到达终点，不能再移动
	ErrMovesIncomplete                // 有未使用的骰子点数，移动不完整
)

const _MaxSteps = 10

// Board 代表一个ludo棋盘
type Board struct {
	Mode     conf.GameModeType // 游戏模式，0=经典模式，1=快速模式等
	pieces   []*Piece          // 所有棋子，id == index
	steps    []*Step           // 执行过的步数
	colorMap map[int32][]int32 // color -> piece ids
}

// NewBoard 创建新的游戏棋盘，seatColors 颜色数组(seat数组)，per 每颜色棋子数
func NewBoard(seatColors []int32, piecesPerSeat int, mode conf.GameModeType) *Board {
	b := &Board{Mode: mode, colorMap: make(map[int32][]int32)}

	var id int32
	for _, color := range seatColors {
		for j := 0; j < piecesPerSeat; j++ {
			p := NewPiece(id, color, mode)
			b.pieces = append(b.pieces, p)
			b.colorMap[color] = append(b.colorMap[color], id)
			id++
		}
	}
	return b
}

func (b *Board) Pieces() []*Piece {
	return b.pieces
}

func (b *Board) Steps() []*Step {
	return b.steps
}

func (b *Board) ColorMap() map[int32][]int32 {
	return b.colorMap
}

func (b *Board) GetPieceByID(id int32) *Piece {
	if id < 0 || int(id) >= len(b.pieces) {
		return nil
	}
	return b.pieces[id]
}

// GetColorPieceIds 返回指定颜色所有棋子Id
func (b *Board) GetColorPieceIds(color int32) []int32 {
	return b.colorMap[color]
}

// ActivePieceIDs 返回指定颜色未到达终点的棋子
func (b *Board) ActivePieceIDs(color int32) []int32 {
	var res []int32
	for _, id := range b.colorMap[color] {
		if p := b.GetPieceByID(id); p == nil || p.IsArrived() {
			continue
		}
		res = append(res, id)
	}
	return res
}

// Clone 深拷贝棋盘
func (b *Board) Clone() *Board {
	c := &Board{
		Mode:     b.Mode,
		pieces:   make([]*Piece, len(b.pieces)),
		steps:    make([]*Step, len(b.steps)),
		colorMap: make(map[int32][]int32),
	}
	for i, p := range b.pieces {
		c.pieces[i] = p.Clone()
	}
	for i, s := range b.steps {
		if s != nil {
			c.steps[i] = s.Clone()
		}
	}
	for k, v := range b.colorMap {
		c.colorMap[k] = v
	}
	return c
}

// Move 单步执行
func (b *Board) Move(pieceID, x int32) (steps *Step) {
	return b.moveOne(pieceID, x)
}

// CanMove 单步执行判断
func (b *Board) CanMove(color, pieceID, x int32) (bool, int32) {
	// 校验持方与移动棋子的持方一致
	p := b.GetPieceByID(pieceID)
	if p == nil {
		return false, ErrInvalidPieceIdx
	}
	if p.color != color {
		return false, ErrInvalidPieceColor
	}
	// 校验是否真的可以移动
	if ok, code, _ := calcNextPos(p.pos, p.color, x); !ok {
		return false, code
	}
	return true, MoveOK
}

// canMoveOne 判断单个棋子能否移动点数x
func (b *Board) canMoveOne(id, x int32) (bool, int32) {
	p := b.GetPieceByID(id)
	if p == nil {
		return false, ErrInvalidPieceIdx
	}
	if ok, code, _ := calcNextPos(p.pos, p.color, x); !ok {
		return false, code
	}
	return true, MoveOK
}

// moveOne 执行单个棋子移动，返回该步详细信息
func (b *Board) moveOne(id, x int32) *Step {
	p := b.GetPieceByID(id)
	if p == nil {
		return nil
	}
	from, to := p.move(x)
	step := &Step{
		Id:     id,
		From:   from,
		To:     to,
		X:      x,
		Color:  p.color,
		Killed: nil,
	}
	if from != to {
		killed := p.canKillAt(b.pieces, to)
		for _, v := range killed {
			// 被击杀后的落点. class模式回到基地, fast模式回到起始点
			basePos := calcBasePos(v.color, b.Mode)
			step.Killed = append(step.Killed, &KilledInfo{
				Id:    v.id,
				From:  v.pos,
				To:    basePos,
				Color: v.color,
			})
			v.setPos(basePos)
		}
	}
	b.steps = append(b.steps, step)
	if len(b.steps) > _MaxSteps {
		copy(b.steps, b.steps[len(b.steps)-_MaxSteps:])
		b.steps = b.steps[:_MaxSteps]
	}
	return step
}

// backOne 撤销最后一步移动
func (b *Board) backOne() *Step {
	if len(b.steps) == 0 {
		return nil
	}
	s := b.steps[len(b.steps)-1]

	// 撤销移动
	if p := b.GetPieceByID(s.Id); p != nil {
		p.setPos(s.From)
	}

	// 撤销击杀
	for _, k := range s.Killed {
		if v := b.GetPieceByID(k.Id); v != nil {
			v.setPos(k.From)
		}
	}

	// 移除最后一步
	b.steps = b.steps[:len(b.steps)-1]
	return s
}

type TagCanMoveDice struct {
	Dice   int32   // 色子点数
	Pieces []int32 // 可移动棋子
}

// CalcCanMoveDice 根据给定颜色和骰子点数，计算所有能移动一步的棋子信息
func (b *Board) CalcCanMoveDice(color int32, dices []int32) []TagCanMoveDice {
	var res []TagCanMoveDice

	pieces := b.ActivePieceIDs(color)
	for _, dice := range dices {
		ps := []int32(nil)
		for _, id := range pieces {
			if can, _ := b.canMoveOne(id, dice); !can {
				continue
			}
			ps = append(ps, id)
		}
		res = append(res, TagCanMoveDice{
			Dice:   dice,
			Pieces: ps,
		})
	}
	return res
}

// // Move 执行批量移动，moves 参数格式为 [pieceID, steps, pieceID, steps, ...]
// // 返回所有移动步骤
// func (b *Board) Move(moves []int32) (steps []*Step) {
// 	for i := 0; i < len(moves); i = i + 2 {
// 		step := b.moveOne(moves[i], moves[i+1])
// 		steps = append(steps, step)
// 	}
// 	return
// }
//
// // CanMove 执行批量移动判断
// // 判断指定棋子能否按色子步数移动, 例如玩家色子[6,6,2] 可以分别移动3次
// // dices: [6,6,2]
// // moves: [pieceID, steps, pieceID, steps, pieceID, steps]
// // len(moves) == len(dices)*2
// func (b *Board) CanMove(color int32, moves []int32) (bool, int32, int32, int32) {
// 	c := b.Clone() // 拷贝棋盘
//
// 	// 逐步校验并模拟移动
// 	for i := 0; i < len(moves); i += 2 {
// 		id := moves[i]
// 		x := moves[i+1]
//
// 		// 校验持方与移动棋子的持方一致
// 		p := c.GetPieceByID(id)
// 		if p == nil {
// 			return false, ErrInvalidPieceIdx, id, x
// 		}
// 		if p.color != color {
// 			return false, ErrInvalidPieceColor, id, x
// 		}
// 		// 校验是否真的可以移动
// 		if ok, code := c.canMoveOne(id, x); !ok {
// 			return false, code, id, x
// 		}
//
// 		c.moveOne(id, x) // 执行移动（仅在克隆板上）
// 	}
//
// 	return true, MoveOK, -1, -1
// }
