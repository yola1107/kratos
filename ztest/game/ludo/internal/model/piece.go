package model

import (
	"fmt"

	"github.com/yola1107/kratos/v2/ztest/game/ludo/internal/conf"
)

// PieceState 表示棋子状态
type PieceState int32

const (
	PieceIdle       PieceState = iota // 未出发
	PieceOnBoard                      // 在公共路径上
	PieceInHomePath                   // 在终点路径上
	PieceArrived                      // 到达终点
)

const (
	ColorCount           = 4  // 4种棋子颜色
	TotalPositions       = 52 // 公共路径长度
	HomePathLen          = 6  // Home路径长度
	BasePos        int32 = -1 // 基地点pos
)

// 导出的棋盘路径参数
var (
	EntryPoints      = []int32{0, 13, 26, 39}      // 四个入口点
	HomeEntrances    = []int32{50, 11, 24, 37}     // 四个回家入口点
	HomeStartIndices = []int32{101, 111, 121, 131} // 四个Home路径起始点
	SafePositions    = map[int32]struct{}{         // 八个安全点（不可被击杀）
		0: {}, 8: {}, 13: {}, 21: {}, 26: {}, 34: {}, 39: {}, 47: {},
	}
)

// Piece 代表一枚棋子
type Piece struct {
	id    int32
	color int32
	pos   int32
	state PieceState
}

func NewPiece(id, color int32, mode conf.GameModeType) *Piece {
	switch mode {
	case conf.ModeFast:
		return &Piece{id: id, color: color, pos: EntryPoints[color], state: PieceOnBoard}
	default:
		return &Piece{id: id, color: color, pos: BasePos, state: PieceIdle}
	}
}

func (p *Piece) Desc() string {
	return fmt.Sprintf("[ID:%d color:%d pos:%d state:%v]", p.id, p.color, p.pos, p.state)
}

func (p *Piece) ID() int32     { return p.id }
func (p *Piece) Color() int32  { return p.color }
func (p *Piece) Pos() int32    { return p.pos }
func (p *Piece) Status() int32 { return int32(p.state) }

func (p *Piece) IsAtEnter() bool       { return p.pos == EntryPoints[p.color] }
func (p *Piece) IsOnBoard() bool       { return p.state == PieceOnBoard }
func (p *Piece) IsArrived() bool       { return p.state == PieceArrived }
func (p *Piece) IsEnemy(o *Piece) bool { return p.color != o.color }

func (p *Piece) Clone() *Piece {
	cp := *p
	return &cp
}

// calcBasePos 计算初始点. class模式初始点在基地. fast模式初始点在起始点(出基地的第一格)
func calcBasePos(color int32, mode conf.GameModeType) int32 {
	switch mode {
	case conf.ModeFast:
		return EntryPoints[color] // 起始点
	default:
		return BasePos // 基地点
	}
}

// setPos 更新位置，自动更新状态
func (p *Piece) setPos(pos int32) {
	p.pos = pos
	p.state = calcStateByPos(pos, p.color)
}

// canKillAt 判断目标位置是否能击杀敌方棋子 (不在安全区才能被吃，叠加了多枚一起吃掉，快速场加分需要*n)
func (p *Piece) canKillAt(pieces []*Piece, targetPos int32) []*Piece {
	if p.state != PieceOnBoard {
		return nil
	}
	if _, ok := SafePositions[targetPos]; ok {
		return nil
	}
	var kills []*Piece
	for _, other := range pieces {
		if other != p && other.pos == targetPos && other.IsOnBoard() && p.IsEnemy(other) {
			kills = append(kills, other)
		}
	}
	return kills
}

// move 移动x
func (p *Piece) move(x int32) (from, to int32) {
	from = p.pos
	can, _, newPos := calcNextPos(p.pos, p.color, x)
	if !can {
		return from, from
	}
	p.setPos(newPos)
	return from, newPos
}

// 根据位置和颜色计算棋子状态
func calcStateByPos(pos, color int32) PieceState {
	if pos == BasePos {
		return PieceIdle
	}
	homeStart := HomeStartIndices[color]
	homeEnd := homeStart + HomePathLen - 1

	switch {
	case pos == homeEnd:
		return PieceArrived
	case pos >= homeStart && pos < homeEnd:
		return PieceInHomePath
	case pos == HomeEntrances[color]:
		return PieceOnBoard // Special state for pieces at home entrance
	case pos >= 0 && pos < TotalPositions:
		return PieceOnBoard
	default:
		return PieceIdle // 容错
	}
}

// 核心计算函数：计算移动步数后的新位置与状态
// 返回是否可移动，错误码，目标位置
func calcNextPos(pos, color, x int32) (bool, int32, int32) {
	if x <= 0 || x > 6 {
		return false, ErrInvalidStep, pos
	}

	homeStart := HomeStartIndices[color]
	homeEnd := homeStart + HomePathLen - 1
	entry := HomeEntrances[color]

	switch {
	case pos == BasePos:
		if x == 6 {
			return true, MoveOK, EntryPoints[color]
		}
		return false, ErrIdleMustBeSix, pos

	case pos == homeEnd:
		return false, ErrAlreadyArrived, pos

	case pos >= homeStart && pos < homeEnd:
		newPos := pos + x
		if newPos <= homeEnd {
			return true, MoveOK, newPos
		}
		return false, ErrExceedHomePath, pos

	case pos >= 0 && pos < TotalPositions:
		distToEntry := (entry - pos + TotalPositions) % TotalPositions
		if x <= distToEntry {
			return true, MoveOK, (pos + x) % TotalPositions
		}
		homeSteps := x - distToEntry - 1
		if homeSteps >= 0 && homeSteps < HomePathLen {
			return true, MoveOK, homeStart + homeSteps
		}
		return false, ErrExceedHomePath, pos

	default:
		return false, ErrInvalidStep, pos
	}
}

// StepsFromStart 返回从 EntryPoint 开始累计的步数，适配快速场或手动设定初始位于 EntryPoint。
// 目前只有快速场需要, 用于算分
func StepsFromStart(pos, color int32) int32 {
	if pos == BasePos {
		return 0
	}

	homeStart := HomeStartIndices[color]

	// Home 路径
	if pos >= homeStart && pos < homeStart+HomePathLen {
		return TotalPositions + (pos - homeStart)
	}

	// 公共路径
	if pos >= 0 && pos < TotalPositions {
		if pos == EntryPoints[color] {
			return 0
		}
		offset := (pos - EntryPoints[color] + TotalPositions) % TotalPositions
		return offset
	}

	return 0
}
