/*
Board 游戏模型

          Game
         /    \
    Board     Players[4]
               |
              Piece[4]

- 共有 52 个公共路径点（0-51）
- 每个颜色有 4 个棋子，起始位置为 -1（未出发）
- 每个颜色的出发点（投6后进入）分别为：
  - Red: 0
  - Green: 13
  - Yellow: 26
  - Blue: 39

- 每个颜色的回家入口点（准备进入 Home 区）分别为：
  - Red: 50
  - Green: 11
  - Yellow: 24
  - Blue: 37

- 每个颜色的 Home 路径起始编号（例如 101~106, 111~116, 121~126, 131~136）分别为：
  - Red: 101
  - Green: 111
  - Yellow: 121
  - Blue: 131

- 安全点（不可被击杀）分别为：
  - 0, 13, 26, 39
  - 8, 21, 34, 47

- 走完所有路径的步数（一般是 57）



Ludo Star 规则核心
基础棋子状态
	Idle（未上板，位置-1）
	OnBoard（走在公共路径上）
	InHomePath（走在Home路径）
	Arrived（到家了）
起始和移动
	骰子投出6时，允许Idle棋子上板，且玩家获得额外一次投骰机会
	玩家最多连续投骰3次（连续6计数）
	其他骰子数时，必须移动OnBoard或InHomePath状态的棋子
	棋子不能走出Home路径终点（比如第57步后必须精确停下）
移动规则
	棋子走6上板后，可以继续走剩余的点数（比如投6和2，6用来上板，2用来走）
	如果多个骰子，允许一次或多次棋子移动，但每次移动消耗一个骰子点数
	不允许跳过Home入口点，进入Home路径后走Home路径编号对应的格子
	如果没棋子能移动，对应骰子点数失效
	只能用完所有骰子点数后才轮到下一玩家
奖励规则:
	这三种情况都可以奖励一次。
	踩到别人的棋子、掷骰子的点数是6、一颗棋子移到终点
连续6
	连续3次投6，玩家本轮直接结束，骰子失效，不走棋


基础规则:
	连续掷出3个6会跳过回合	    真实规则中连续3次掷6会触发惩罚
	只有 6 才能出发	        但如果已经在外，其他骰子都能动
	打人回家、吃子优先	        同一位置能站1枚棋子，撞上送回家
	安全区不可被吃         	星点位是安全区，不能被打人
	入 home path 时不能超出	点数精确时才能进入终点
	特殊奖励（掷6再投）	    走完还能再掷一次

流程:
	每个骰子数都可以单独走一次，也就是说，你可以选择先走6步，再走6步，最后走2步，总共3次移动
	玩家轮次开始 ->
	投骰子状态 -> 玩家投骰子（可能多次） ->
	移动状态 -> 玩家用骰子点数走棋子（逐步消耗点数） ->
	结束轮次，切换到下一个玩家

| 颜色 | 起点位置 | 回家入口 | Home 路径起点 | Home 终点 |
| -- | ----     | ----    | --------- | ------- |
| 红  | 0       | 50       | 101       | 106    |
| 绿  | 13      | 11       | 111       | 116    |
| 黄  | 26      | 24       | 121       | 126    |
| 蓝  | 39      | 37       | 131       | 136    |


积分规则:
	棋子每前进1格得分+1 。
	每次踩到对方棋子踩方得分+20，被踩方则会减掉该棋子已移动格数（比如：对手的棋子从起始格开始已经移动了30格，这时被另外一方踩中，则踩方+20，被踩方-30）。
	将棋子移动到主区域（棋牌中心），每个棋子可获得“ +50 ”分
	如果中途有玩家退出，则该玩家得为将重置为0。



快速场模式行为区别（和经典模式对比）
	项目	经典模式	快速模式
	初始位置	BasePos（-1）	EntryPoints[color]
	起步条件	必须投 6 才能出	起始就可以移动
	状态初始化	PieceIdle	PieceOnBoard
	移动起始步数	从第 0 步	从第 1 步（已在 entry）
	StepsFromStart	从 EntryPoint+X 算	同上
*/

package model

import (
	"fmt"
	"strings"
	"testing"
)

func TestCalcNextPos1(t *testing.T) {
	pos := int32(48)
	step := int32(5)
	color := int32(0)

	ok, code, to := calcNextPos(pos, color, step)
	t.Log(ok, code, to)
}

func TestCalcNextPos(t *testing.T) {
	type args struct {
		pos   int32
		color int32
		step  int32
	}
	tests := []struct {
		name     string
		args     args
		wantOk   bool
		wantCode int32
		wantTo   int32
	}{
		// 出发测试
		{"BasePos invalid step", args{BasePos, 0, 5}, false, ErrIdleMustBeSix, BasePos},
		{"BasePos valid 6", args{BasePos, 1, 6}, true, MoveOK, EntryPoints[1]},

		// 普通路径前进
		{"Public path normal", args{14, 1, 3}, true, MoveOK, 17},
		{"Public path wrap", args{50, 0, 3}, true, MoveOK, 103},

		// 进入 Home 区入口（color=0，entry=50，homeStart=101）
		{"Enter Home exact", args{48, 0, 3}, true, MoveOK, 101},
		{"Enter Home overshoot", args{48, 0, 5}, true, MoveOK, 103},
		{"Enter Home invalid overshoot", args{48, 0, 8}, false, ErrInvalidStep, 48},

		// 已在 Home 路径内
		{"Home path normal", args{101, 0, 2}, true, MoveOK, 103},
		{"Home path reach end", args{101, 0, 5}, true, MoveOK, 106},
		{"Home path overflow", args{101, 0, 6}, false, ErrExceedHomePath, 101},

		// 已到终点
		{"Already arrived", args{106, 0, 1}, false, ErrAlreadyArrived, 106},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotOk, gotCode, gotTo := calcNextPos(tt.args.pos, tt.args.color, tt.args.step)
			if gotOk != tt.wantOk || gotCode != tt.wantCode || gotTo != tt.wantTo {
				t.Errorf("calcNextPos() = (ok:%v, code:%v, to:%v), want (ok:%v, code:%v, to:%v)",
					gotOk, gotCode, gotTo, tt.wantOk, tt.wantCode, tt.wantTo)
			}
		})
	}
}

func TestNewBoard(t *testing.T) {
	b := NewBoard([]int32{0, 1, 3}, 4, false)
	if b == nil {
		t.Error("NewBoard returned nil")
		return
	}

	c := b.Clone()
	defer c.Clear()
	if c == nil {
		t.Error("Clone returned nil")
		return
	}

	t.Logf("%p %p\n", b, c)
	t.Logf("b= %p %+v\n", b, b.Desc())
	t.Logf("c= %p %+v\n", c, c.Desc())
}

func (b *Board) Desc() string {
	var builder strings.Builder
	builder.WriteString(fmt.Sprintf("Board with %d pieces and %d steps\n", len(b.pieces), len(b.steps)))
	for i, p := range b.pieces {
		builder.WriteString(fmt.Sprintf("  Piece %d: PieceID=%d color=%d Pos=%d State=%d\n",
			i, p.id, p.color, p.pos, p.state))
	}
	return builder.String()
}

// | 名称          | color | EntryPos | Pos | 期望 StepsFromStart |
// | ----------- | ----- | -------- | --- | ----------------- |
// | 红色\_起点      | 0     | 0        | 0   | 0                 |
// | 红色\_走一步     | 0     | 0        | 1   | 1                 |
// | 红色\_最大走位    | 0     | 0        | 51  | 51                |
// | 红色\_Home起点  | 0     | 0        | 101 | 53                |
// | 红色\_Home终点  | 0     | 0        | 106 | 58                |
// | 黄色\_起点      | 2     | 26       | 26  | 0                 |
// | 黄色\_走回0     | 2     | 26       | 0   | 26                |
// | 黄色\_Wrap到25 | 2     | 26       | 25  | 51                |
func TestStepsFromStart_Table(t *testing.T) {
	tests := []struct {
		name  string
		color int32
		pos   int32
		want  int32
	}{
		{"红色_起点", 0, 0, 0},
		{"红色_走一步", 0, 1, 1},
		{"红色_回家入口点", 0, 50, 50},
		{"红色_最大走位", 0, 51, 51}, // 实际不会有51
		{"红色_Home起点", 0, 101, 52},
		{"红色_Home终点", 0, 106, 57},
		{"黄色_起点", 2, 26, 0},
		{"黄色_走回0", 2, 0, 26},
		{"黄色_Wrap到25", 2, 25, 51},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := StepsFromStart(tt.pos, tt.color)
			if got != tt.want {
				t.Errorf("StepsFromStart(pos=%v, color=%v) = %v, want %v",
					tt.pos, tt.color, got, tt.want)
			}
		})
	}
}
func TestStepsFromStart(t *testing.T) {
	type args struct {
		color int32
		pos   int32
	}
	tests := []struct {
		name string
		args args
		want int32
	}{
		// 基地
		{"BasePos", args{0, BasePos}, 0},

		// 刚进入 EntryPoint
		{"AtEntryPoint_Red", args{0, EntryPoints[0]}, 0},
		{"AtEntryPoint_Yellow", args{2, EntryPoints[2]}, 0},

		// 公共路径正常前进
		{"MoveOne_Red", args{0, EntryPoints[0] + 1}, 1},
		{"MoveFive_Red", args{0, EntryPoints[0] + 5}, 5},

		// wrap-around：红色从 Entry=0，走到 51，应为 51
		{"WrapAround_Red", args{0, 51}, 51},
		// wrap-around：绿色 Entry=13，当前位置=3 应为 (3 - 13 + 52) % 52 = 42
		{"WrapAround_Green", args{1, 3}, 42},

		// Home 路径
		{"HomeStart", args{0, HomeStartIndices[0]}, 52},
		{"HomeMid", args{0, HomeStartIndices[0] + 3}, 55},
		{"HomeEnd", args{0, HomeStartIndices[0] + 5}, 57},

		// 非法位置
		{"InvalidNegative", args{0, -99}, 0},
		{"InvalidTooLarge", args{0, 999}, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := StepsFromStart(tt.args.pos, tt.args.color)
			if got != tt.want {
				t.Errorf("StepsFromStart(color=%v, pos=%v) = %v, want %v",
					tt.args.color, tt.args.pos, got, tt.want)
			}
		})
	}
}
