package model

import (
	"fmt"
	"time"
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
	visited := make([]bool, len(dices))
	b.permute(color, []int32(nil), ret, dices, visited, len(dices), all, ids)

	if false {
		fmt.Printf("Permute. use:%v/ms color:%+v dices:%+v ret:%+v\n", time.Since(start).Milliseconds(), color, dices, ret.Desc())
	}
	return
}

func (b *Board) permute(color int32, cache []int32, _Ret *TagRetData, dices []int32, visited []bool, max int, all bool, ids []int32) {
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

		for _, v := range ids {
			moverID := v
			if ok, _ := b.canMoveOne(moverID, dices[j]); !ok {
				continue
			}

			visited[j] = true
			cache = append(cache, moverID, dices[j])
			b.moveOne(moverID, dices[j])
			b.permute(color, cache, _Ret, dices, visited, max, all, ids)
			b.backOne()
			cache = cache[:len(cache)-2]
			visited[j] = false
		}
	}
}
